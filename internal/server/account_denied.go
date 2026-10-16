package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/niclasedge/tollgate/internal/pipeline"
)

// accountDeniedRetryAfter is long enough for a manual account switch to land
// within Claude Code's default ten retries (about 2.5 minutes).
const accountDeniedRetryAfter = "15"

// maxDeniedBody bounds what is read of an upstream 403 to classify it.
const maxDeniedBody = 64 << 10

// accountDenied reports the OAuth error code when an upstream 403 says the
// active account may no longer use OAuth (e.g. oauth_not_allowed_for_organization).
// Switching the account fixes that, so the client should retry, not give up.
func accountDenied(raw []byte) (string, bool) {
	var e struct {
		Error struct {
			Type    string `json:"type"`
			Details struct {
				Code string `json:"error_code"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &e) != nil || e.Error.Type != "permission_error" {
		return "", false
	}
	code := e.Error.Details.Code
	return code, strings.HasPrefix(code, "oauth_")
}

// handleAccountDenied answers an upstream 403 on /v1/messages. It returns
// false with resp.Body restored when the 403 is not an account denial.
func (s *Server) handleAccountDenied(w http.ResponseWriter, resp *http.Response, req *pipeline.Request, accountID string) bool {
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxDeniedBody))
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(raw), resp.Body), resp.Body}
	if err != nil {
		return false
	}
	code, ok := accountDenied(raw)
	if !ok {
		return false
	}
	s.log.Warn("upstream account not allowed", "trace_id", req.TraceID, "account_id", accountID, "error_code", code)
	w.Header().Set("Retry-After", accountDeniedRetryAfter)
	w.Header().Set("X-Should-Retry", "true")
	writeStageError(w, &pipeline.StageError{Stage: "forward", Status: http.StatusServiceUnavailable, Type: pipeline.ErrTypeAPI,
		Message: "active account is not allowed to use OAuth (" + code + "); switch the account in tollgate, retrying"})
	return true
}

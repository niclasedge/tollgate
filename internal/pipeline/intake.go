package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/go-json-experiment/json"
	"github.com/niclasedge/tollgate/internal/jsonsplice"
)

// StageIntake is the name of stage 0 in traces.
const StageIntake = "intake"

// ErrInvalidKey is what a KeyVerifier returns for an unknown, blocked or
// malformed client key. Any other error is an internal failure (500).
var ErrInvalidKey = errors.New("invalid client key")

// KeyVerifier checks a presented client key and returns the client ID.
// The real implementation (SHA-256 lookup, constant-time compare, ADR-0009)
// comes with #8; stage 0 only depends on this abstraction.
type KeyVerifier interface {
	Verify(ctx context.Context, key string) (clientID string, err error)
}

// ConfigPinner returns the configuration version a session is pinned to
// (ADR-0006/0008). sessionKey is empty for "client without session"; the
// pinner then returns the currently active version. Optional: without a
// pinner the version stays 0. SessionPinner is the store-backed implementation.
type ConfigPinner interface {
	Pin(ctx context.Context, clientID, sessionKey string) (version int64, err error)
}

// Intake is stage 0: client key check, request parsing, session key and
// configuration version. It never produces replacements.
type Intake struct {
	Keys   KeyVerifier
	Pinner ConfigPinner // optional
}

// Name implements Stage.
func (*Intake) Name() string { return StageIntake }

// Authenticate checks the client key in h and returns the client ID. It
// needs only the headers, so the server calls it before reading the body
// (#52); Process repeats it so the stage stays self-contained.
func (in *Intake) Authenticate(ctx context.Context, h http.Header) (string, error) {
	if c, ok := internalClient(ctx); ok {
		return c.ID, nil // set in-process only (#87)
	}
	key := clientKey(h)
	if key == "" {
		return "", authError()
	}
	clientID, err := in.Keys.Verify(ctx, key)
	switch {
	case errors.Is(err, ErrInvalidKey):
		return "", authError()
	case err != nil:
		return "", &StageError{Status: http.StatusInternalServerError, Type: ErrTypeAPI,
			Message: "internal error in stage " + StageIntake, Cause: err}
	}
	return clientID, nil
}

// Process implements Stage.
func (in *Intake) Process(ctx context.Context, req *Request) ([]jsonsplice.Replacement, error) {
	clientID, err := in.Authenticate(ctx, req.Header)
	if err != nil {
		return nil, err
	}
	req.ClientID = clientID
	req.Endpoint = classify(req.Method, req.Path)

	sessionID := ""
	if sid := strings.TrimSpace(req.Header.Get("X-Claude-Code-Session-Id")); validSessionID(sid) {
		sessionID, req.SessionSource = sid, SessionFromHeader
	}
	if req.Endpoint != EndpointPassthrough {
		doc, err := jsonsplice.Scan(req.Body)
		if err != nil {
			return nil, &StageError{Status: http.StatusBadRequest, Type: ErrTypeInvalidRequest,
				Message: "request body is not valid JSON", Cause: err}
		}
		req.Model, req.Stream = doc.Model, doc.Stream
		if sessionID == "" {
			if sid := sessionFromUserID(doc.MetadataUserID); validSessionID(sid) {
				sessionID, req.SessionSource = sid, SessionFromMetadata
			}
		}
	}
	if sessionID != "" {
		h := sha256.Sum256([]byte(clientID + "\x00" + sessionID))
		req.SessionKey = hex.EncodeToString(h[:])
	}
	if in.Pinner != nil {
		if req.ConfigVersion, err = in.Pinner.Pin(ctx, clientID, req.SessionKey); err != nil {
			return nil, &StageError{Status: http.StatusInternalServerError, Type: ErrTypeAPI,
				Message: "internal error in stage " + StageIntake, Cause: err}
		}
	}
	return nil, nil
}

func authError() *StageError {
	return &StageError{Status: http.StatusUnauthorized, Type: ErrTypeAuthentication,
		Message: "invalid or missing client key"}
}

// clientKey reads the key from "Authorization: Bearer", otherwise from
// "x-api-key" (an Authorization header with another scheme is ignored).
func clientKey(h http.Header) string {
	scheme, rest, ok := strings.Cut(h.Get("Authorization"), " ")
	if ok && strings.EqualFold(scheme, "Bearer") {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(h.Get("X-Api-Key"))
}

func classify(method, path string) Endpoint {
	if method == http.MethodPost {
		switch path {
		case "/v1/messages":
			return EndpointMessages
		case "/v1/messages/count_tokens":
			return EndpointCountTokens
		}
	}
	return EndpointPassthrough
}

func validSessionID(s string) bool { return s != "" && len(s) <= 256 }

// sessionFromUserID extracts the session part of metadata.user_id. Two shapes
// are understood: a JSON object string with "session_id" (verified against
// Claude Code 2.1.289, see testdata/fixtures and the issue-41 design.md), and
// the legacy "user_<hash>_account_<uuid>_session_<uuid>" (not observed, fallback).
func sessionFromUserID(userID string) string {
	userID = strings.TrimSpace(userID)
	if strings.HasPrefix(userID, "{") {
		var v struct {
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal([]byte(userID), &v) != nil {
			return ""
		}
		return v.SessionID
	}
	if i := strings.LastIndex(userID, "_session_"); i >= 0 {
		return userID[i+len("_session_"):]
	}
	return ""
}

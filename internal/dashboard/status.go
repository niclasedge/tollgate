package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/niclasedge/tollgate/internal/meter"
	"github.com/niclasedge/tollgate/internal/upstream"
)

// statusTimeout bounds all calls to CLIProxyAPI of one status request.
const statusTimeout = 3 * time.Second

// status reports tollgate and CLIProxyAPI health and the accounts of the auth
// dir. It is always 200: an unreachable upstream or a missing management key
// is described in the messages. Only the fields of Account are taken from the
// management API; tokens and file contents never pass through.
func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	out := Status{
		AccountManagement: h.accounts != nil,
		Tollgate:          ServiceHealth{OK: true, Version: h.version},
		Accounts:          []Account{},
	}
	if h.limits != nil {
		if l, ok := h.limits(); ok {
			out.Limits = toRateLimits(l)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), statusTimeout)
	defer cancel()
	if err := h.up.Health(ctx); err != nil {
		out.CLIProxyAPI = ServiceHealth{Message: "CLIProxyAPI nicht erreichbar"}
		writeJSON(w, http.StatusOK, out)
		return
	}
	out.CLIProxyAPI = ServiceHealth{OK: true}
	if h.mgmtKey == "" {
		out.CLIProxyAPI.Message = "Accounts nicht verfügbar: kein Management-Key konfiguriert"
		writeJSON(w, http.StatusOK, out)
		return
	}
	creds, err := h.up.Credentials(ctx, h.mgmtKey)
	if err != nil {
		out.CLIProxyAPI.Message = "Accounts nicht verfügbar: Management-API antwortet nicht"
		writeJSON(w, http.StatusOK, out)
		return
	}
	now := h.now()
	for _, c := range creds {
		out.Accounts = append(out.Accounts, toAccount(c, now))
	}
	switch n := len(out.Accounts); {
	case n == 0:
		out.CLIProxyAPI.Message = "Kein Account im auth-dir"
	case n > 1:
		out.CLIProxyAPI.Message = fmt.Sprintf("Warnung: %d Accounts im auth-dir, erwartet wird genau einer", n)
	}
	writeJSON(w, http.StatusOK, out)
}

func toAccount(c upstream.Credential, now time.Time) Account {
	a := Account{Name: c.Name, Provider: c.Provider, Email: c.Email, Status: c.Status, Disabled: c.Disabled}
	if c.NextRetryAfter.After(now) {
		a.CooldownUntil = formatTime(c.NextRetryAfter)
	}
	return a
}

func toRateLimits(l meter.Limits) *RateLimits {
	return &RateLimits{
		Status: l.Status, Reset: formatOptTime(l.Reset), RepresentativeClaim: l.RepresentativeClaim,
		FiveHour: toWindow(l.FiveHour), SevenDay: toWindow(l.SevenDay), ObservedAt: formatTime(l.ObservedAt),
	}
}

func toWindow(w meter.Window) LimitWindow {
	return LimitWindow{Utilization: w.Utilization, Reset: formatOptTime(w.Reset), Status: w.Status}
}

func formatOptTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

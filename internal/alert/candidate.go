package alert

import (
	"cmp"
	"context"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/niclasedge/tollgate/internal/accounts"
)

type candidate struct {
	account accounts.Account
	week    *float64 // percent; nil: unknown
	five    *float64
}

// level is the utilization in percent; a window whose reset has passed is
// empty again.
func level(w accounts.UsageWindow, now time.Time) *float64 {
	if w.Utilization == nil {
		return nil
	}
	if r, err := time.Parse(time.RFC3339, w.Reset); err == nil && !r.After(now) {
		zero := 0.0
		return &zero
	}
	v := *w.Utilization
	return &v
}

// pickCandidates returns the accounts to switch to, best first: not active,
// not the affected one, no new login needed, week and 5 hours below 100 % or
// unknown.
func pickCandidates(ctx context.Context, acc Accounts, list []accounts.Account, exclude string, now time.Time) []candidate {
	var out []candidate
	for _, a := range list {
		if a.Active || a.ID == exclude {
			continue
		}
		u, err := acc.Usage(ctx, a.ID)
		if err != nil {
			u = accounts.UsageSnapshot{}
		}
		if u.License != nil && u.License.Token == accounts.TokenRelogin {
			continue
		}
		c := candidate{account: a, week: level(u.SevenDay, now), five: level(u.FiveHour, now)}
		if (c.week != nil && *c.week >= 100) || (c.five != nil && *c.five >= 100) {
			continue
		}
		out = append(out, c)
	}
	or0 := func(p *float64) float64 {
		if p == nil {
			return 0
		}
		return *p
	}
	slices.SortFunc(out, func(a, b candidate) int {
		if (a.week == nil) != (b.week == nil) {
			if a.week == nil {
				return 1
			}
			return -1
		}
		return cmp.Or(
			cmp.Compare(or0(a.five), or0(b.five)),
			cmp.Compare(or0(a.week), or0(b.week)),
			cmp.Compare(a.account.Name, b.account.Name),
			cmp.Compare(a.account.ID, b.account.ID),
		)
	})
	return out
}

func itoa(n int) string { return strconv.Itoa(n) }

func percent(p *float64) string {
	if p == nil {
		return "unbekannt"
	}
	return itoa(int(math.Floor(*p+1e-9))) + " %"
}

func (c candidate) describe() string {
	return "Woche " + percent(c.week) + ", 5 h " + percent(c.five)
}

func alternative(cands []candidate) string {
	if len(cands) == 0 {
		return "Kein anderes Abo mit freiem Wochenlimit."
	}
	return "Ausweichen: *" + cands[0].account.Name + "* (" + cands[0].describe() + ")."
}

// resetText is " (Reset in 1 h 20 min)", empty without a future reset.
func resetText(reset *time.Time, now time.Time) string {
	if reset == nil || !reset.After(now) {
		return ""
	}
	m := int(reset.Sub(now) / time.Minute)
	switch {
	case m < 1:
		return " (Reset in < 1 min)"
	case m < 60:
		return " (Reset in " + itoa(m) + " min)"
	default:
		return " (Reset in " + itoa(m/60) + " h " + itoa(m%60) + " min)"
	}
}

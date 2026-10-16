package redactcfg

import (
	"strings"

	"github.com/niclasedge/tollgate/internal/store"
)

// maxDiffCells bounds the LCS table; larger changes fall back to a full
// replacement of the changed middle.
const maxDiffCells = 4_000_000

// diffTruncated ends a diff that hit the store limit.
const diffTruncated = "… gekürzt"

// lineDiff returns the changed lines between a and b as "- old" / "+ new",
// in order, without context. Equal inputs give "". The result is
// deterministic and capped below store.MaxDiffBytes.
func lineDiff(a, b string) string {
	x, y := splitLines(a), splitLines(b)
	p := 0
	for p < len(x) && p < len(y) && x[p] == y[p] {
		p++
	}
	s := 0
	for s < len(x)-p && s < len(y)-p && x[len(x)-1-s] == y[len(y)-1-s] {
		s++
	}
	x, y = x[p:len(x)-s], y[p:len(y)-s]

	var out []string
	if len(x)*len(y) > maxDiffCells {
		for _, l := range x {
			out = append(out, "- "+l)
		}
		for _, l := range y {
			out = append(out, "+ "+l)
		}
	} else {
		out = lcsDiff(x, y)
	}
	return capDiff(out)
}

func lcsDiff(x, y []string) []string {
	n, m := len(x), len(y)
	// t[i][j] = LCS length of x[i:] and y[j:].
	t := make([][]int32, n+1)
	for i := range t {
		t[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if x[i] == y[j] {
				t[i][j] = t[i+1][j+1] + 1
			} else {
				t[i][j] = max(t[i+1][j], t[i][j+1])
			}
		}
	}
	var out []string
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case x[i] == y[j]:
			i, j = i+1, j+1
		case t[i+1][j] >= t[i][j+1]:
			out = append(out, "- "+x[i])
			i++
		default:
			out = append(out, "+ "+y[j])
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, "- "+x[i])
	}
	for ; j < m; j++ {
		out = append(out, "+ "+y[j])
	}
	return out
}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func capDiff(lines []string) string {
	limit := store.MaxDiffBytes - len(diffTruncated) - 1
	var b strings.Builder
	for _, l := range lines {
		if b.Len()+len(l)+1 > limit {
			b.WriteString(diffTruncated + "\n")
			break
		}
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

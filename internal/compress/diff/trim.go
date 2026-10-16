// Context trimming.
//
// Source: Headroom crates/headroom-core/src/transforms/diff_compressor.rs
// (reduce_context). Headroom (Apache-2.0, Copyright 2025 Headroom
// Contributors), Stand 0947a28, portiert und verändert.
//
// Deviations: the "@@" header is left byte-identical, so its counts no longer
// match a trimmed hunk; the marker line reports the trimmed context lines
// instead. Hunks without any changed line are left alone (Headroom keeps only
// their first lines).

package diff

// trimContext marks unchanged lines farther than ctx lines from the nearest
// changed line. Lines starting with a backslash ("\ No newline at end of
// file") stay with the line before them. It returns the number of lines trimmed.
func trimContext(d *document, ctx int) int {
	total := 0
	for _, b := range d.blocks {
		if b.f == nil || b.f.dropped {
			continue
		}
		for _, h := range b.f.hunks {
			if h.drop != keepHunk {
				continue
			}
			total += h.trimContext(ctx)
		}
	}
	return total
}

func (h *hunk) trimContext(ctx int) int {
	n := len(h.lines)
	near := make([]bool, n)
	any := false
	for i, l := range h.lines {
		if !h.isChange(l) {
			continue
		}
		any = true
		lo, hi := i-ctx, i+ctx
		if lo < 0 {
			lo = 0
		}
		if hi > n-1 {
			hi = n - 1
		}
		for j := lo; j <= hi; j++ {
			near[j] = true
		}
	}
	if !any {
		return 0
	}
	h.trim = make([]bool, n)
	trimmed := 0
	for i, l := range h.lines {
		if l != "" && l[0] == '\\' {
			// A "\ No newline" marker belongs to the line before it.
			near[i] = i == 0 || !h.trim[i-1]
		}
		if !near[i] {
			h.trim[i] = true
			trimmed++
		}
	}
	return trimmed
}

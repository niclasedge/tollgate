// Caps for files and hunks.
//
// Source: Headroom crates/headroom-core/src/transforms/diff_compressor.rs
// (file cap in compress_with_store, select_hunks, score_hunks, priority_patterns).
// Headroom (Apache-2.0, Copyright 2025 Headroom Contributors), Stand 0947a28,
// portiert und verändert.
//
// Deviations: no query scoring. The hunk score is the change density plus the
// priority pattern boost, as integers (percent) so no float rounding is
// involved. Files over the cap are dropped by change count but the kept files
// stay in input order (Headroom re-sorts them by size). A cap of 1 keeps only
// the first hunk (Headroom would keep first and last).

package diff

import (
	"regexp"
	"sort"
	"strings"
)

const (
	scoreDensityWeight = 3  // percent per changed line
	scoreDensityCap    = 30 // percent
	scorePriorityBoost = 30 // percent, once per hunk
)

// priorityPatterns are Headroom's PRIORITY_PATTERNS_DIFF (error, importance,
// security words), applied to the lower-cased hunk body.
var priorityPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b(error|exception|fail(?:ed|ure)?|fatal|critical|crash|panic)\b`),
	regexp.MustCompile(`\b(important|note|todo|fixme|hack|xxx|bug|fix)\b`),
	regexp.MustCompile(`\b(security|auth|password|secret|token)\b`),
}

func (h *hunk) score() int {
	s := h.changes() * scoreDensityWeight
	if s > scoreDensityCap {
		s = scoreDensityCap
	}
	body := strings.ToLower(strings.Join(h.lines, "\n"))
	for _, re := range priorityPatterns {
		if re.MatchString(body) {
			s += scorePriorityBoost
			break
		}
	}
	return s
}

// limitFacts are what the limits step removed.
type limitFacts struct {
	files []string // paths of dropped files
	hunks int      // hunks dropped by the per-file cap
}

// applyLimits drops the smallest files beyond maxFiles and, per file, the
// least interesting hunks beyond maxHunks. Files without a live hunk (binary,
// rename only, lockfile) do not count toward the file cap.
func applyLimits(d *document, maxFiles, maxHunks int) limitFacts {
	var lf limitFacts

	var cand []*file
	for _, b := range d.blocks {
		if b.f != nil && len(b.f.liveHunks()) > 0 {
			cand = append(cand, b.f)
		}
	}
	if len(cand) > maxFiles {
		order := make([]int, len(cand))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(a, b int) bool {
			return cand[order[a]].changes() > cand[order[b]].changes()
		})
		for _, i := range order[maxFiles:] {
			cand[i].dropped = true
		}
		for _, f := range cand { // input order
			if f.dropped {
				lf.files = appendUnique(lf.files, f.path)
			}
		}
	}

	for _, f := range cand {
		if f.dropped {
			continue
		}
		live := f.liveHunks()
		if len(live) <= maxHunks {
			continue
		}
		keep := selectHunks(live, maxHunks)
		for i, h := range live {
			if !keep[i] {
				h.drop = dropLimit
				lf.hunks++
			}
		}
	}
	return lf
}

// selectHunks keeps the first hunk, the last hunk and the best scoring
// middle hunks (ties: earlier first); it returns a keep flag per hunk.
func selectHunks(live []*hunk, max int) []bool {
	keep := make([]bool, len(live))
	keep[0] = true
	if max == 1 {
		return keep
	}
	keep[len(live)-1] = true
	middle := make([]int, 0, len(live)-2)
	for i := 1; i < len(live)-1; i++ {
		middle = append(middle, i)
	}
	scores := make(map[int]int, len(middle)) // lookup only, never iterated
	for _, i := range middle {
		scores[i] = live[i].score()
	}
	sort.SliceStable(middle, func(a, b int) bool { return scores[middle[a]] > scores[middle[b]] })
	slots := max - 2
	if slots > len(middle) {
		slots = len(middle)
	}
	for _, i := range middle[:slots] {
		keep[i] = true
	}
	return keep
}

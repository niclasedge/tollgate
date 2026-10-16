package meter

import (
	"strings"

	"github.com/niclasedge/tollgate/internal/store"
)

// Price is the list price of a model in nano-USD per token, which is the same
// number as USD per million tokens times 1000 ($4 / MTok = 4000). Integers
// keep cost sums exact and deterministic.
type Price struct {
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64 // 5-minute cache write
}

// prices are the Anthropic API list prices (platform.claude.com/docs/en/
// about-claude/pricing, as of 2026-10-09), keyed by model ID prefix. The
// longest matching prefix wins, so "claude-opus-5-5" beats "claude-opus-5".
// Claude Haiku 5.5 costs 5x for prompts over 100K tokens; reports sum usage
// before pricing, so only the lower tier is listed and its cost is a lower
// bound (#138).
var prices = map[string]Price{
	"claude-fable-5-1":  {Input: 10000, Output: 50000, CacheRead: 250, CacheWrite: 12500},
	"claude-mythos-5-1": {Input: 10000, Output: 50000, CacheRead: 250, CacheWrite: 12500},
	"claude-fable-5":    {Input: 10000, Output: 50000, CacheRead: 1000, CacheWrite: 12500},
	"claude-mythos-5":   {Input: 10000, Output: 50000, CacheRead: 1000, CacheWrite: 12500},
	"claude-opus-5-5":   {Input: 4000, Output: 20000, CacheRead: 200, CacheWrite: 5000},
	"claude-opus-5":     {Input: 5000, Output: 25000, CacheRead: 500, CacheWrite: 6250},
	"claude-opus-4-8":   {Input: 5000, Output: 25000, CacheRead: 500, CacheWrite: 6250},
	"claude-opus-4-7":   {Input: 5000, Output: 25000, CacheRead: 500, CacheWrite: 6250},
	"claude-opus-4-6":   {Input: 5000, Output: 25000, CacheRead: 500, CacheWrite: 6250},
	"claude-opus-4-5":   {Input: 5000, Output: 25000, CacheRead: 500, CacheWrite: 6250},
	"claude-opus-4-1":   {Input: 15000, Output: 75000, CacheRead: 1500, CacheWrite: 18750},
	"claude-opus-4":     {Input: 15000, Output: 75000, CacheRead: 1500, CacheWrite: 18750},
	"claude-sonnet-5-5": {Input: 2000, Output: 10000, CacheRead: 100, CacheWrite: 2500},
	"claude-sonnet-5":   {Input: 2000, Output: 10000, CacheRead: 200, CacheWrite: 2500},
	"claude-sonnet-4-6": {Input: 3000, Output: 15000, CacheRead: 300, CacheWrite: 3750},
	"claude-sonnet-4-5": {Input: 3000, Output: 15000, CacheRead: 300, CacheWrite: 3750},
	"claude-sonnet-4":   {Input: 3000, Output: 15000, CacheRead: 300, CacheWrite: 3750},
	"claude-haiku-5-5":  {Input: 100, Output: 500, CacheRead: 10, CacheWrite: 125},
	"claude-haiku-4-5":  {Input: 1000, Output: 5000, CacheRead: 100, CacheWrite: 1250},
	"claude-3-5-haiku":  {Input: 800, Output: 4000, CacheRead: 80, CacheWrite: 1000},
}

// PriceFor returns the list price of model. A prefix only matches at a
// boundary ("-", "[" or the end), so "claude-opus-4" does not price
// "claude-opus-40". ok is false for unknown models.
func PriceFor(model string) (Price, bool) {
	best := ""
	for prefix := range prices {
		if len(prefix) <= len(best) || !strings.HasPrefix(model, prefix) {
			continue
		}
		if rest := model[len(prefix):]; rest != "" && rest[0] != '-' && rest[0] != '[' {
			continue
		}
		best = prefix
	}
	if best == "" {
		return Price{}, false
	}
	return prices[best], true
}

// Cost returns the list-price cost of u in nano-USD. Cache writes are priced
// at the 5-minute rate because usage does not split them by duration, so the
// value is a lower bound when 1-hour caching is used. Other multipliers (US
// inference 1.1x, fast mode, batch) are not applied either. ok is false for
// unknown models.
func Cost(model string, u store.Usage) (nanoUSD int64, ok bool) {
	p, ok := PriceFor(model)
	if !ok {
		return 0, false
	}
	return u.Input*p.Input + u.Output*p.Output + u.CacheRead*p.CacheRead + u.CacheCreation*p.CacheWrite, true
}

// CacheSavings returns what reading u.CacheRead tokens from the cache saved
// against paying the full input price, in nano-USD. Cache writes cost more
// than plain input and are not counted. ok is false for unknown models.
func CacheSavings(model string, u store.Usage) (nanoUSD int64, ok bool) {
	p, ok := PriceFor(model)
	if !ok {
		return 0, false
	}
	return u.CacheRead * (p.Input - p.CacheRead), true
}

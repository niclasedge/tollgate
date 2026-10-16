// Package redactcfg holds the versioned runtime configuration (ADR-0008
// level 2): schema validation, YAML import and export, versions with diff and
// rollback in the store, hot reload, dry runs on fixtures and the admin API.
//
// The word list is confidential (ADR-0009): it is stored only sealed, shown
// masked, never logged and never quoted in error messages.
package redactcfg

import (
	"bytes"

	"github.com/niclasedge/tollgate/internal/compress"
	"github.com/niclasedge/tollgate/internal/compress/strat"
	"github.com/niclasedge/tollgate/internal/redact"
	"go.yaml.in/yaml/v3"
)

// Document is one runtime configuration. The field order is the export
// order.
type Document struct {
	Version   int          `yaml:"version" json:"version"`
	Builtin   Builtin      `yaml:"builtin" json:"builtin"`
	Custom    []CustomRule `yaml:"custom" json:"custom"`
	Words     Words        `yaml:"words" json:"words"`
	Allowlist []string     `yaml:"allowlist" json:"allowlist"`
	Compress  *Compress    `yaml:"compress,omitempty" json:"compress,omitempty"`
}

// Builtin switches the built-in detector groups (ADR-0005).
type Builtin struct {
	Email          bool `yaml:"email" json:"email"`
	APIKeys        bool `yaml:"api_keys" json:"api_keys"`
	JWT            bool `yaml:"jwt" json:"jwt"`
	ConnectionURLs bool `yaml:"connection_urls" json:"connection_urls"`
	IPAddresses    bool `yaml:"ip_addresses" json:"ip_addresses"`
	PrivateKeys    bool `yaml:"private_keys" json:"private_keys"`
}

// CustomRule is a user-defined regex filter.
type CustomRule struct {
	Name        string   `yaml:"name" json:"name"`
	Pattern     string   `yaml:"pattern" json:"pattern"`
	Placeholder string   `yaml:"placeholder" json:"placeholder"`
	Examples    []string `yaml:"examples" json:"examples"`
}

// Words holds the confidential word lists: the general list (placeholder
// type word) and the lists of customers (kunde), first names (vorname) and
// last names (nachname, #143). All four share CaseSensitive and WholeWord.
type Words struct {
	CaseSensitive bool     `yaml:"case_sensitive" json:"case_sensitive"`
	WholeWord     bool     `yaml:"whole_word" json:"whole_word"`
	List          []string `yaml:"list" json:"list"`
	Customers     []string `yaml:"customers" json:"customers"`
	FirstNames    []string `yaml:"first_names" json:"first_names"`
	LastNames     []string `yaml:"last_names" json:"last_names"`
}

// lists returns the four lists in YAML and export order.
func (w *Words) lists() [4]*[]string {
	return [4]*[]string{&w.List, &w.Customers, &w.FirstNames, &w.LastNames}
}

// listKeys are the YAML keys of Words.lists.
var listKeys = [4]string{"list", "customers", "first_names", "last_names"}

// any reports whether one of the four lists has an entry.
func (w Words) any() bool {
	for _, l := range w.lists() {
		if len(*l) > 0 {
			return true
		}
	}
	return false
}

// Compress is validated and versioned here; #19 evaluates it.
type Compress struct {
	MinBytes *int           `yaml:"min_bytes,omitempty" json:"min_bytes,omitempty"`
	Rules    []CompressRule `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// CompressRule selects a strategy for Bash commands with a given prefix.
type CompressRule struct {
	Name         string   `yaml:"name" json:"name"`
	Match        []string `yaml:"match" json:"match"`
	Strategy     string   `yaml:"strategy" json:"strategy"`
	MaxErrors    *int     `yaml:"max_errors,omitempty" json:"max_errors,omitempty"`
	ContextLines *int     `yaml:"context_lines,omitempty" json:"context_lines,omitempty"`
	MaxFiles     *int     `yaml:"max_files,omitempty" json:"max_files,omitempty"`
	MaxHunks     *int     `yaml:"max_hunks,omitempty" json:"max_hunks,omitempty"`
}

// Default returns the built-in default configuration: redact.DefaultOptions,
// no custom rules, no words, no allowlist, no compress section.
func Default() *Document {
	o := redact.DefaultOptions()
	d := &Document{
		Version: 1,
		Builtin: Builtin{Email: o.Email, APIKeys: o.APIKeys, JWT: o.JWT, ConnectionURLs: o.ConnectionURLs,
			IPAddresses: o.IPAddresses, PrivateKeys: o.PrivateKeys},
		Words: Words{CaseSensitive: false, WholeWord: true},
	}
	d.normalize()
	return d
}

// normalize makes absent and empty lists equal so that import and export
// round-trip to the same value.
func (d *Document) normalize() {
	if d.Custom == nil {
		d.Custom = []CustomRule{}
	}
	for _, l := range d.Words.lists() {
		if *l == nil {
			*l = []string{}
		}
	}
	if d.Allowlist == nil {
		d.Allowlist = []string{}
	}
	if d.Compress != nil && len(d.Compress.Rules) == 0 {
		d.Compress.Rules = nil
	}
}

// clone returns a deep copy (the word lists included).
func (d *Document) clone() *Document {
	c := *d
	c.Custom = make([]CustomRule, len(d.Custom))
	for i, r := range d.Custom {
		r.Examples = append([]string(nil), r.Examples...)
		c.Custom[i] = r
	}
	cl, dl := c.Words.lists(), d.Words.lists()
	for i := range cl {
		*cl[i] = append([]string{}, *dl[i]...)
	}
	c.Allowlist = append([]string{}, d.Allowlist...)
	if d.Compress != nil {
		cp := *d.Compress
		cp.MinBytes = cloneInt(cp.MinBytes)
		cp.Rules = nil
		for _, r := range d.Compress.Rules {
			r.Match = append([]string(nil), r.Match...)
			r.MaxErrors, r.ContextLines = cloneInt(r.MaxErrors), cloneInt(r.ContextLines)
			r.MaxFiles, r.MaxHunks = cloneInt(r.MaxFiles), cloneInt(r.MaxHunks)
			cp.Rules = append(cp.Rules, r)
		}
		c.Compress = &cp
	}
	return &c
}

func cloneInt(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// RedactConfig returns the redact part for redact.Compile.
func (d *Document) RedactConfig() redact.Config {
	c := redact.Config{
		Builtin: redact.Options{Email: d.Builtin.Email, APIKeys: d.Builtin.APIKeys, JWT: d.Builtin.JWT,
			ConnectionURLs: d.Builtin.ConnectionURLs, IPAddresses: d.Builtin.IPAddresses, PrivateKeys: d.Builtin.PrivateKeys},
		Words: redact.WordList{CaseSensitive: d.Words.CaseSensitive, WholeWord: d.Words.WholeWord, List: d.Words.List,
			Customers: d.Words.Customers, FirstNames: d.Words.FirstNames, LastNames: d.Words.LastNames},
		Allowlist: d.Allowlist,
	}
	for _, r := range d.Custom {
		c.Custom = append(c.Custom, redact.CustomRule{Name: r.Name, Pattern: r.Pattern, Placeholder: r.Placeholder, Examples: r.Examples})
	}
	return c
}

// CompressConfig returns the compress rules of the document (#19): the
// configured rules first, then the built-in ones; without a compress section
// only the built-in ones.
func (d *Document) CompressConfig() *compress.Config {
	if d.Compress == nil {
		return compress.Default()
	}
	rules := make([]compress.Rule, 0, len(d.Compress.Rules))
	for _, r := range d.Compress.Rules {
		rules = append(rules, compress.Rule{Name: r.Name, Match: r.Match, Strategy: r.Strategy, Params: strat.Params{
			MaxErrors: deref(r.MaxErrors), ContextLines: deref(r.ContextLines), MaxFiles: deref(r.MaxFiles), MaxHunks: deref(r.MaxHunks)}})
	}
	return compress.WithDefaults(d.Compress.MinBytes, rules)
}

func deref(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// ExportMode selects how the word lists appear in an export.
type ExportMode int

const (
	// ExportOmitWords writes every word list as `[]`. Default of the API.
	ExportOmitWords ExportMode = iota
	// ExportPlainWords writes the word lists in plain text after a warning.
	ExportPlainWords
	// ExportMaskedWords masks every entry; for display and diffs only, the
	// result is not importable.
	ExportMaskedWords
)

// Header comments of the export modes.
const (
	PlainWordsWarning  = "# WARNUNG: enthält die vertrauliche Wortliste – nicht committen"
	omitWordsComment   = "# Wortlisten nicht enthalten (words.list, customers, first_names, last_names: []); Export mit words=plain enthält sie."
	maskedWordsComment = "# Wortlisten maskiert – nicht importierbar."
)

// Export writes d deterministically: fixed field order, two-space indent,
// every switch written out, empty lists as [].
func Export(d *Document, mode ExportMode) []byte {
	c := d.clone()
	c.normalize()
	header := omitWordsComment
	switch mode {
	case ExportPlainWords:
		header = PlainWordsWarning
	case ExportMaskedWords:
		header = maskedWordsComment
		for _, l := range c.Words.lists() {
			for i, w := range *l {
				(*l)[i] = Mask(w)
			}
		}
	default:
		for _, l := range c.Words.lists() {
			*l = []string{}
		}
	}
	var buf bytes.Buffer
	buf.WriteString(header + "\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		panic("redactcfg: encode document: " + err.Error()) // plain structs always encode
	}
	_ = enc.Close()
	return buf.Bytes()
}

// maskDots is the fixed filler of Mask; its length is independent of the
// word (ADR-0009: "Länge nicht exakt").
const maskDots = "••••••••"

// Mask hides a word-list entry (of any of the four lists): first and last character around a fixed
// filler for entries of at least six characters, otherwise only the filler.
func Mask(w string) string {
	r := []rune(w)
	if len(r) < 6 {
		return maskDots
	}
	return string(r[0]) + maskDots + string(r[len(r)-1])
}

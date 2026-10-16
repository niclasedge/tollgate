package redact

import (
	"fmt"
	"regexp"
	"strings"
)

// Placeholder types of the word lists (#143): the general list and the lists
// of customers, first names and last names.
const (
	TypeWord      = "word"
	TypeCustomer  = "kunde"
	TypeFirstName = "vorname"
	TypeLastName  = "nachname"
)

// IsListType reports whether typ is the placeholder type of a confidential
// word list. Such values are never shown, not even partly, and never
// revealed.
func IsListType(typ string) bool {
	switch typ {
	case TypeWord, TypeCustomer, TypeFirstName, TypeLastName:
		return true
	}
	return false
}

// Priorities of configurable detectors (ADR-0005: word list and custom
// rules before every built-in rule). The word list is the explicit list of
// confidential names, so it wins over custom rules.
const (
	prioCustom = 100
	prioWords  = 110
)

// Config is the redact part of the runtime configuration (ADR-0008 level 2).
type Config struct {
	Builtin   Options
	Custom    []CustomRule
	Words     WordList
	Allowlist []string
}

// CustomRule is a user-defined detector. The whole match is the secret.
type CustomRule struct {
	Name        string
	Pattern     string // RE2 syntax
	Placeholder string // placeholder type (ADR-0006 grammar)
	Examples    []string
}

// WordList holds confidential words such as company or project names. The
// four lists share CaseSensitive and WholeWord and differ in the placeholder
// type: List is "word", Customers "kunde", FirstNames "vorname", LastNames
// "nachname".
type WordList struct {
	CaseSensitive bool
	WholeWord     bool
	List          []string
	Customers     []string
	FirstNames    []string
	LastNames     []string
}

// ConfigError is one semantic problem. Path uses the YAML field names, e.g.
// "custom[0].examples[1]". Msg never quotes a word-list entry.
type ConfigError struct {
	Path string
	Msg  string
}

// ConfigErrors lists all problems of a configuration in field order.
type ConfigErrors []ConfigError

func (e ConfigErrors) Error() string {
	parts := make([]string, len(e))
	for i, c := range e {
		parts[i] = c.Path + ": " + c.Msg
	}
	return strings.Join(parts, "; ")
}

var (
	customTypeRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	nameRE       = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

// reservedTypes are the placeholder types of the built-in rules and the word
// list; custom rules must not reuse them.
var reservedTypes = []string{TypeEmail, TypeAPIKey, TypeJWT, TypeConnectionPassword, TypeIP, TypePrivateKey, TypeWord}

// Compile validates c and returns an immutable Redactor with the built-in
// switches, custom rules, word list and allowlist. Errors are ConfigErrors.
// The result does not depend on map iteration order: custom rules keep their
// configured order, words are sorted (longer first, then lexicographic).
func Compile(c Config) (*Redactor, error) {
	r := New(c.Builtin)
	var errs ConfigErrors
	add := func(path, format string, args ...any) {
		errs = append(errs, ConfigError{Path: path, Msg: fmt.Sprintf(format, args...)})
	}
	order := len(builtinRules)

	names := make(map[string]int, len(c.Custom))
	for i, cr := range c.Custom {
		p := fmt.Sprintf("custom[%d]", i)
		if !nameRE.MatchString(cr.Name) {
			add(p+".name", "muss %s entsprechen", nameRE.String())
		} else if j, dup := names[cr.Name]; dup {
			add(p+".name", "Name ist schon in custom[%d] vergeben", j)
		} else {
			names[cr.Name] = i
		}
		if !customTypeRE.MatchString(cr.Placeholder) {
			add(p+".placeholder", "muss %s entsprechen", customTypeRE.String())
		} else {
			for _, t := range reservedTypes {
				if cr.Placeholder == t {
					add(p+".placeholder", "kollidiert mit dem eingebauten Typ %s", t)
				}
			}
		}
		re, err := regexp.Compile(cr.Pattern)
		if err != nil {
			add(p+".pattern", "Regex ungültig: %v", err)
			continue
		}
		if re.MatchString("") {
			add(p+".pattern", "Regex darf nicht auf leeren Text passen")
			continue
		}
		if len(cr.Examples) == 0 {
			add(p+".examples", "mindestens ein Beispiel ist Pflicht")
		}
		for j, ex := range cr.Examples {
			if loc := re.FindStringIndex(ex); loc == nil || loc[0] == loc[1] {
				add(fmt.Sprintf("%s.examples[%d]", p, j), "Beispiel trifft das Muster nicht")
			}
		}
		r.rules = append(r.rules, indexedRule{
			rule:  rule{name: "custom:" + cr.Name, re: re, typ: cr.Placeholder, priority: prioCustom},
			order: order,
		})
		order++
	}

	// The rule order is fixed: it decides between equal matches of different
	// lists.
	for _, l := range []struct {
		path, rule, typ string
		list            []string
	}{
		{"words.list", "words", TypeWord, c.Words.List},
		{"words.customers", "customers", TypeCustomer, c.Words.Customers},
		{"words.first_names", "first_names", TypeFirstName, c.Words.FirstNames},
		{"words.last_names", "last_names", TypeLastName, c.Words.LastNames},
	} {
		for i, w := range l.list {
			if strings.TrimSpace(w) == "" {
				add(fmt.Sprintf("%s[%d]", l.path, i), "Eintrag ist leer")
			}
		}
		for _, w := range sortedWords(l.list) {
			r.rules = append(r.rules, indexedRule{rule: wordRule(w, c.Words, l.rule, l.typ), order: order})
			order++
		}
	}

	if len(c.Allowlist) > 0 {
		r.allow = make(map[string]struct{}, len(c.Allowlist))
		for _, v := range c.Allowlist {
			r.allow[v] = struct{}{}
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return r, nil
}

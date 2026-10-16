package redactcfg

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/niclasedge/tollgate/config/schema"
	"github.com/niclasedge/tollgate/internal/redact"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"go.yaml.in/yaml/v3"
)

// Issue is one validation problem. Path uses YAML field names
// ("custom[0].placeholder", "" for the document root). Message is German,
// user-facing and never quotes a word-list entry.
type Issue struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}

func (i Issue) String() string {
	p := i.Path
	if p == "" {
		p = "(Dokument)"
	}
	return fmt.Sprintf("%s (Zeile %d, Spalte %d): %s", p, i.Line, i.Column, i.Message)
}

// ValidationError lists every problem found in one stage of validation
// (YAML syntax, schema, semantics), sorted by position.
type ValidationError struct {
	Issues []Issue
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Issues))
	for i, is := range e.Issues {
		parts[i] = is.String()
	}
	return strings.Join(parts, "; ")
}

// maxNodes bounds alias expansion ("billion laughs").
const maxNodes = 200_000

var compiledSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema.ConfigV1))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(schema.URL, doc); err != nil {
		return nil, err
	}
	return c.Compile(schema.URL)
})

// Parse reads and fully validates a YAML configuration: syntax, JSON Schema
// and semantics (redact.Compile, compress rules). Validation errors are
// *ValidationError.
func Parse(data []byte) (*Document, error) {
	d, _, err := parse(data)
	return d, err
}

func parse(data []byte) (*Document, *redact.Redactor, error) {
	root, is := decodeYAML(data)
	if is != nil {
		return nil, nil, &ValidationError{Issues: []Issue{*is}}
	}
	budget := maxNodes
	inst, is := toValue(root, &budget)
	if is != nil {
		return nil, nil, &ValidationError{Issues: []Issue{*is}}
	}
	sch, err := compiledSchema()
	if err != nil {
		return nil, nil, fmt.Errorf("redactcfg: schema: %w", err)
	}
	if err := sch.Validate(inst); err != nil {
		var ve *jsonschema.ValidationError
		if !errors.As(err, &ve) {
			return nil, nil, fmt.Errorf("redactcfg: validate: %w", err)
		}
		return nil, nil, &ValidationError{Issues: sortIssues(schemaIssues(root, ve))}
	}

	d := Default()
	if err := root.Decode(d); err != nil {
		// The schema already checked all types; never surface the decoder
		// message, it may quote values.
		return nil, nil, &ValidationError{Issues: []Issue{{Line: root.Line, Column: root.Column, Message: "Dokument lässt sich nicht lesen"}}}
	}
	d.normalize()

	var issues []Issue
	r, err := redact.Compile(d.RedactConfig())
	var ce redact.ConfigErrors
	if errors.As(err, &ce) {
		for _, c := range ce {
			issues = append(issues, issueAt(root, splitPath(c.Path), c.Msg))
		}
	} else if err != nil {
		return nil, nil, err
	}
	issues = append(issues, compressIssues(root, d)...)
	for k, l := range d.Words.lists() {
		for i, w := range *l {
			if strings.Contains(w, maskDots) {
				issues = append(issues, issueAt(root, []string{"words", listKeys[k], strconv.Itoa(i)},
					"maskierter Eintrag aus einer Versionsansicht – nicht importierbar"))
			}
		}
	}
	if len(issues) > 0 {
		return nil, nil, &ValidationError{Issues: sortIssues(issues)}
	}
	return d, r, nil
}

var yamlLineRE = regexp.MustCompile(`^yaml: line (\d+): (.*)$`)

// decodeYAML returns the root mapping node of exactly one YAML document.
func decodeYAML(data []byte) (*yaml.Node, *Issue) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, &Issue{Line: 1, Column: 1, Message: "Dokument ist leer"}
		}
		return nil, syntaxIssue(err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err == nil {
		return nil, &Issue{Line: extra.Line, Column: extra.Column, Message: "mehrere YAML-Dokumente sind nicht erlaubt"}
	} else if !errors.Is(err, io.EOF) {
		return nil, syntaxIssue(err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, &Issue{Line: 1, Column: 1, Message: "Dokument ist leer"}
	}
	root := resolve(doc.Content[0])
	if root.Kind != yaml.MappingNode {
		return nil, &Issue{Line: root.Line, Column: root.Column, Message: "die oberste Ebene muss ein Objekt mit Schlüsseln sein"}
	}
	return root, nil
}

func syntaxIssue(err error) *Issue {
	if m := yamlLineRE.FindStringSubmatch(err.Error()); m != nil {
		line, _ := strconv.Atoi(m[1])
		return &Issue{Line: line, Column: 1, Message: "YAML-Syntaxfehler: " + m[2]}
	}
	return &Issue{Line: 1, Column: 1, Message: "YAML-Syntaxfehler: " + strings.TrimPrefix(err.Error(), "yaml: ")}
}

func resolve(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

// toValue converts a node into JSON-compatible values for the schema check.
// Duplicate keys are rejected; aliases are expanded within a node budget.
func toValue(n *yaml.Node, budget *int) (any, *Issue) {
	n = resolve(n)
	if *budget--; *budget < 0 {
		return nil, &Issue{Line: n.Line, Column: n.Column, Message: "Dokument ist zu groß (zu viele Knoten oder Aliase)"}
	}
	switch n.Kind {
	case yaml.MappingNode:
		m := make(map[string]any, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := resolve(n.Content[i])
			if k.Kind != yaml.ScalarNode {
				return nil, &Issue{Line: k.Line, Column: k.Column, Message: "Schlüssel muss ein einfacher Text sein"}
			}
			if _, dup := m[k.Value]; dup {
				return nil, &Issue{Line: k.Line, Column: k.Column, Message: fmt.Sprintf("Schlüssel %q ist doppelt", k.Value)}
			}
			v, is := toValue(n.Content[i+1], budget)
			if is != nil {
				return nil, is
			}
			m[k.Value] = v
		}
		return m, nil
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, is := toValue(c, budget)
			if is != nil {
				return nil, is
			}
			out = append(out, v)
		}
		return out, nil
	case yaml.ScalarNode:
		switch n.ShortTag() {
		case "!!null":
			return nil, nil
		case "!!bool":
			var b bool
			if err := n.Decode(&b); err == nil {
				return b, nil
			}
		case "!!int":
			var i int64
			if err := n.Decode(&i); err == nil {
				return i, nil
			}
			var f float64
			if err := n.Decode(&f); err == nil {
				return f, nil
			}
		case "!!float":
			var f float64
			if err := n.Decode(&f); err == nil {
				return f, nil
			}
		}
		return n.Value, nil
	}
	return nil, &Issue{Line: n.Line, Column: n.Column, Message: "nicht unterstützter YAML-Knoten"}
}

// locate walks the instance path through the node tree and returns the
// display path and the deepest node found.
func locate(root *yaml.Node, tokens []string) (string, *yaml.Node) {
	var path strings.Builder
	n := root
	for _, tok := range tokens {
		n = resolve(n)
		var next *yaml.Node
		switch n.Kind {
		case yaml.MappingNode:
			if _, v := mapEntry(n, tok); v != nil {
				next = v
				if path.Len() > 0 {
					path.WriteByte('.')
				}
				path.WriteString(tok)
			}
		case yaml.SequenceNode:
			if i, err := strconv.Atoi(tok); err == nil && i >= 0 && i < len(n.Content) {
				next = n.Content[i]
				fmt.Fprintf(&path, "[%d]", i)
			}
		}
		if next == nil {
			break
		}
		n = next
	}
	return path.String(), n
}

func mapEntry(n *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if k := resolve(n.Content[i]); k.Value == key {
			return k, n.Content[i+1]
		}
	}
	return nil, nil
}

func issueAt(root *yaml.Node, tokens []string, msg string) Issue {
	path, n := locate(root, tokens)
	return Issue{Path: path, Line: n.Line, Column: n.Column, Message: msg}
}

// splitPath turns "custom[0].examples[1]" into tokens.
func splitPath(p string) []string {
	var out []string
	for _, part := range strings.Split(p, ".") {
		name, rest, _ := strings.Cut(part, "[")
		if name != "" {
			out = append(out, name)
		}
		for rest != "" {
			idx, after, _ := strings.Cut(rest, "]")
			out = append(out, idx)
			rest = strings.TrimPrefix(after, "[")
		}
	}
	return out
}

func schemaIssues(root *yaml.Node, ve *jsonschema.ValidationError) []Issue {
	if len(ve.Causes) > 0 {
		var out []Issue
		for _, c := range ve.Causes {
			out = append(out, schemaIssues(root, c)...)
		}
		return out
	}
	is := issueAt(root, ve.InstanceLocation, describe(ve.ErrorKind))
	if ap, ok := ve.ErrorKind.(*kind.AdditionalProperties); ok && len(ap.Properties) > 0 {
		// Point at the unknown key itself.
		_, n := locate(root, ve.InstanceLocation)
		if k, _ := mapEntry(resolve(n), ap.Properties[0]); k != nil {
			is.Line, is.Column = k.Line, k.Column
		}
	}
	return []Issue{is}
}

// describe renders a schema error in German without quoting the checked
// value (it may be a word-list entry).
func describe(k jsonschema.ErrorKind) string {
	switch k := k.(type) {
	case *kind.Type:
		return "muss vom Typ " + strings.Join(k.Want, " oder ") + " sein"
	case *kind.Required:
		return "Pflichtfeld fehlt: " + strings.Join(k.Missing, ", ")
	case *kind.AdditionalProperties:
		return "unbekanntes Feld: " + strings.Join(k.Properties, ", ")
	case *kind.Const:
		return fmt.Sprintf("muss %v sein", k.Want)
	case *kind.Enum:
		want := make([]string, len(k.Want))
		for i, w := range k.Want {
			want[i] = fmt.Sprint(w)
		}
		return "muss einer dieser Werte sein: " + strings.Join(want, ", ")
	case *kind.Pattern:
		return "muss " + k.Want + " entsprechen"
	case *kind.Not:
		return "kollidiert mit einem eingebauten Platzhalter-Typ"
	case *kind.MinLength:
		if k.Want == 1 {
			return "darf nicht leer sein"
		}
		return fmt.Sprintf("muss mindestens %d Zeichen lang sein", k.Want)
	case *kind.MaxLength:
		return fmt.Sprintf("darf höchstens %d Zeichen lang sein", k.Want)
	case *kind.MinItems:
		return fmt.Sprintf("braucht mindestens %d Einträge", k.Want)
	case *kind.MaxItems:
		return fmt.Sprintf("darf höchstens %d Einträge haben", k.Want)
	case *kind.Minimum:
		return "muss mindestens " + k.Want.RatString() + " sein"
	case *kind.UniqueItems:
		return fmt.Sprintf("Einträge %d und %d sind gleich", k.Duplicates[0], k.Duplicates[1])
	case *kind.FalseSchema:
		return "ist nicht erlaubt"
	}
	return "ungültig (" + strings.Join(k.KeywordPath(), "/") + ")"
}

func compressIssues(root *yaml.Node, d *Document) []Issue {
	if d.Compress == nil {
		return nil
	}
	var out []Issue
	names := map[string]int{}
	for i, r := range d.Compress.Rules {
		if j, dup := names[r.Name]; dup {
			out = append(out, issueAt(root, []string{"compress", "rules", strconv.Itoa(i), "name"},
				fmt.Sprintf("Name doppelt (auch in compress.rules[%d])", j)))
		} else {
			names[r.Name] = i
		}
		for j, m := range r.Match {
			if strings.TrimSpace(m) == "" {
				out = append(out, issueAt(root, []string{"compress", "rules", strconv.Itoa(i), "match", strconv.Itoa(j)},
					"Präfix ist leer"))
			}
		}
	}
	return out
}

func sortIssues(is []Issue) []Issue {
	sort.SliceStable(is, func(i, j int) bool {
		a, b := is[i], is[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Message < b.Message
	})
	return is
}

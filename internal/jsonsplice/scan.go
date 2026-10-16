package jsonsplice

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

// Kind says which part of a message a target string belongs to.
type Kind string

const (
	// KindMessageContent is messages[i].content given as a plain string.
	KindMessageContent Kind = "message_content"
	// KindText is the text field of a "text" block.
	KindText Kind = "text"
	// KindToolResult is a tool_result content string or a text block inside it.
	KindToolResult Kind = "tool_result"
	// KindToolUseInput is any string value (recursively) of a tool_use input.
	KindToolUseInput Kind = "tool_use_input"
)

// Target is one string a stage may rewrite. Start and End delimit the full
// string token including quotes, Value is its decoded content.
type Target struct {
	Kind         Kind   `json:"kind"`
	Role         string `json:"role"`
	MessageIndex int    `json:"message_index"`
	// BlockIndex is -1 for plain-string message content.
	BlockIndex int `json:"block_index"`
	// ToolID links tool calls and results (#19): the id of a tool_use for
	// its input strings, the tool_use_id of a tool_result for its content.
	ToolID string `json:"tool_id,omitempty"`
	// ToolName is the name of the tool_use for its input strings.
	ToolName string `json:"tool_name,omitempty"`
	Path     string `json:"path"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Value    string `json:"-"`
}

// Document is the result of one Scan pass.
type Document struct {
	// Targets are in document order (ascending Start, never overlapping).
	Targets        []Target
	Model          string
	Stream         bool
	MetadataUserID string
}

// Scan validates body as strict JSON (RFC 8259, no duplicate member names,
// valid UTF-8, no trailing data) and collects the rewritable strings.
//
// Only user/assistant text, tool_result content and tool_use input strings are
// reported (ADR-0006 block table). thinking, redacted_thinking, signature,
// compaction, system, tools, metadata, images, documents and every unknown
// block type are never visited as targets, so they cannot be altered.
func Scan(body []byte) (*Document, error) {
	s := &scanner{dec: jsontext.NewDecoder(bytes.NewReader(body)), doc: &Document{}}
	if k, err := s.peek(); err != nil {
		return nil, wrap(err)
	} else if k != '{' {
		return nil, errNotObject
	}
	if err := s.top(); err != nil {
		return nil, wrap(err)
	}
	if _, err := s.dec.ReadToken(); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing data after JSON value")
		}
		return nil, wrap(err)
	}
	return s.doc, nil
}

func wrap(err error) error {
	return fmt.Errorf("jsonsplice: invalid request body: %w", err)
}

type scanner struct {
	dec *jsontext.Decoder
	doc *Document
}

// peek returns the kind of the next value; at end of input or on a syntax
// error it returns the underlying decoder error.
func (s *scanner) peek() (jsontext.Kind, error) {
	k := s.dec.PeekKind()
	if k == 0 {
		if _, err := s.dec.ReadToken(); err != nil {
			return 0, err
		}
		return 0, errors.New("unexpected end of input")
	}
	return k, nil
}

func (s *scanner) name() (string, error) {
	t, err := s.dec.ReadToken()
	if err != nil {
		return "", err
	}
	return t.String(), nil
}

// str reads a string value and returns its token range and decoded value.
func (s *scanner) str() (start, end int, val string, err error) {
	raw, err := s.dec.ReadValue()
	if err != nil {
		return 0, 0, "", err
	}
	end = int(s.dec.InputOffset())
	start = end - len(raw)
	if err := json.Unmarshal(raw, &val); err != nil {
		return 0, 0, "", err
	}
	return start, end, val, nil
}

// object iterates the members of the next object value, calling fn with each
// member name. fn must consume the member value.
func (s *scanner) object(fn func(name string) error) error {
	if _, err := s.dec.ReadToken(); err != nil { // '{'
		return err
	}
	for {
		k, err := s.peek()
		if err != nil {
			return err
		}
		if k == '}' {
			_, err = s.dec.ReadToken()
			return err
		}
		name, err := s.name()
		if err != nil {
			return err
		}
		if err := fn(name); err != nil {
			return err
		}
	}
}

// array iterates the elements of the next array value, calling fn with the
// element index. fn must consume the element.
func (s *scanner) array(fn func(i int) error) error {
	if _, err := s.dec.ReadToken(); err != nil { // '['
		return err
	}
	for i := 0; ; i++ {
		k, err := s.peek()
		if err != nil {
			return err
		}
		if k == ']' {
			_, err = s.dec.ReadToken()
			return err
		}
		if err := fn(i); err != nil {
			return err
		}
	}
}

func (s *scanner) top() error {
	return s.object(func(name string) error {
		k, err := s.peek()
		if err != nil {
			return err
		}
		switch {
		case name == "model" && k == '"':
			_, _, s.doc.Model, err = s.str()
			return err
		case name == "stream" && (k == 't' || k == 'f'):
			t, err := s.dec.ReadToken()
			s.doc.Stream = err == nil && t.Bool()
			return err
		case name == "metadata" && k == '{':
			return s.object(func(name string) error {
				k, err := s.peek()
				if err != nil {
					return err
				}
				if name == "user_id" && k == '"' {
					_, _, s.doc.MetadataUserID, err = s.str()
					return err
				}
				return s.dec.SkipValue()
			})
		case name == "messages":
			if k != '[' {
				return errors.New("messages is not an array")
			}
			return s.array(s.message)
		}
		return s.dec.SkipValue()
	})
}

func (s *scanner) message(i int) error {
	k, err := s.peek()
	if err != nil {
		return err
	}
	if k != '{' {
		return s.dec.SkipValue()
	}
	prefix := "messages[" + strconv.Itoa(i) + "]"
	var cands []Target
	role := ""
	err = s.object(func(name string) error {
		k, err := s.peek()
		if err != nil {
			return err
		}
		switch {
		case name == "role" && k == '"':
			_, _, role, err = s.str()
			return err
		case name == "content" && k == '"':
			start, end, val, err := s.str()
			cands = append(cands, Target{Kind: KindMessageContent, MessageIndex: i, BlockIndex: -1,
				Path: prefix + ".content", Start: start, End: end, Value: val})
			return err
		case name == "content" && k == '[':
			return s.array(func(j int) error {
				got, err := s.block(i, j, prefix+".content["+strconv.Itoa(j)+"]")
				cands = append(cands, got...)
				return err
			})
		}
		return s.dec.SkipValue()
	})
	if err != nil {
		return err
	}
	for _, c := range cands {
		c.Role = role
		s.doc.Targets = append(s.doc.Targets, c)
	}
	return nil
}

// block reads one content block. Candidate strings are collected while
// scanning because "type" may follow the fields; afterwards only the
// candidates matching the block type survive.
func (s *scanner) block(i, j int, path string) ([]Target, error) {
	k, err := s.peek()
	if err != nil {
		return nil, err
	}
	if k != '{' {
		return nil, s.dec.SkipValue()
	}
	typ, id, toolUseID, toolName := "", "", "", ""
	var cands []Target
	err = s.object(func(name string) error {
		k, err := s.peek()
		if err != nil {
			return err
		}
		base := Target{MessageIndex: i, BlockIndex: j}
		switch {
		case name == "type" && k == '"':
			_, _, typ, err = s.str()
			return err
		case name == "id" && k == '"':
			_, _, id, err = s.str()
			return err
		case name == "tool_use_id" && k == '"':
			_, _, toolUseID, err = s.str()
			return err
		case name == "name" && k == '"':
			_, _, toolName, err = s.str()
			return err
		case name == "text" && k == '"':
			t := base
			t.Kind, t.Path = KindText, path+".text"
			t.Start, t.End, t.Value, err = s.str()
			cands = append(cands, t)
			return err
		case name == "content" && k == '"':
			t := base
			t.Kind, t.Path = KindToolResult, path+".content"
			t.Start, t.End, t.Value, err = s.str()
			cands = append(cands, t)
			return err
		case name == "content" && k == '[':
			return s.array(func(n int) error {
				got, err := s.resultItem(base, path+".content["+strconv.Itoa(n)+"]")
				cands = append(cands, got...)
				return err
			})
		case name == "input" && k == '{':
			got, err := s.inputStrings(base, []byte(path+".input"))
			cands = append(cands, got...)
			return err
		}
		return s.dec.SkipValue()
	})
	if err != nil {
		return nil, err
	}
	var want Kind
	switch typ {
	case "text":
		want = KindText
	case "tool_result":
		want = KindToolResult
	case "tool_use":
		want = KindToolUseInput
	default:
		return nil, nil
	}
	out := cands[:0]
	for _, c := range cands {
		if c.Kind != want {
			continue
		}
		switch want {
		case KindToolUseInput:
			c.ToolID, c.ToolName = id, toolName
		case KindToolResult:
			c.ToolID = toolUseID
		}
		out = append(out, c)
	}
	return out, nil
}

// resultItem reads one element of a tool_result content array; only
// {"type":"text","text":...} elements yield a target.
func (s *scanner) resultItem(base Target, path string) ([]Target, error) {
	k, err := s.peek()
	if err != nil {
		return nil, err
	}
	if k != '{' {
		return nil, s.dec.SkipValue()
	}
	typ := ""
	var cand *Target
	err = s.object(func(name string) error {
		k, err := s.peek()
		if err != nil {
			return err
		}
		switch {
		case name == "type" && k == '"':
			_, _, typ, err = s.str()
			return err
		case name == "text" && k == '"':
			t := base
			t.Kind, t.Path = KindToolResult, path+".text"
			t.Start, t.End, t.Value, err = s.str()
			cand = &t
			return err
		}
		return s.dec.SkipValue()
	})
	if err != nil || cand == nil || typ != "text" {
		return nil, err
	}
	return []Target{*cand}, nil
}

// inputStrings collects every string value (not member names) of a tool_use
// input, recursively, in document order. path is a shared buffer that is only
// copied into a string at a target, so deep nesting costs linear memory.
func (s *scanner) inputStrings(base Target, path []byte) ([]Target, error) {
	k, err := s.peek()
	if err != nil {
		return nil, err
	}
	var out []Target
	switch k {
	case '"':
		t := base
		t.Kind, t.Path = KindToolUseInput, string(path)
		t.Start, t.End, t.Value, err = s.str()
		return append(out, t), err
	case '{':
		err = s.object(func(name string) error {
			got, err := s.inputStrings(base, append(append(path, '.'), name...))
			out = append(out, got...)
			return err
		})
	case '[':
		err = s.array(func(n int) error {
			got, err := s.inputStrings(base, append(strconv.AppendInt(append(path, '['), int64(n), 10), ']'))
			out = append(out, got...)
			return err
		})
	default:
		err = s.dec.SkipValue()
	}
	return out, err
}

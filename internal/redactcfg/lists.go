package redactcfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// sealedWords is the plain text of the sealed blob of a version: all four
// word lists as one object (#143). Blobs written before are a JSON array
// holding only words.list; decodeSealed reads both.
type sealedWords struct {
	List       []string `json:"list"`
	Customers  []string `json:"customers"`
	FirstNames []string `json:"first_names"`
	LastNames  []string `json:"last_names"`
}

func encodeSealed(w Words) ([]byte, error) {
	return json.Marshal(sealedWords{List: w.List, Customers: w.Customers, FirstNames: w.FirstNames, LastNames: w.LastNames})
}

func decodeSealed(plain []byte) (sealedWords, error) {
	var s sealedWords
	if p := bytes.TrimSpace(plain); len(p) > 0 && p[0] == '[' {
		return s, json.Unmarshal(p, &s.List)
	}
	return s, json.Unmarshal(plain, &s)
}

// nameLists returns the three name lists of w (never nil).
func (w Words) nameLists() NameLists {
	return NameLists{
		Customers:  append([]string{}, w.Customers...),
		FirstNames: append([]string{}, w.FirstNames...),
		LastNames:  append([]string{}, w.LastNames...),
	}
}

// applyListsSet cleans the lists of ls for use as lists of a draft: entries
// trimmed, empty entries and duplicates (per caseSensitive, folded like the
// redactor's (?i) rules) dropped, input order kept, limits as in the schema.
// Lists that are absent stay absent. Violations are a *ValidationError with
// issues at lists_set.<list>[i], which never quote an entry. The result
// depends on the input only; present lists are never nil.
func applyListsSet(ls ListsSet, caseSensitive bool) (ListsSet, error) {
	var out ListsSet
	var issues []Issue
	for _, l := range []struct {
		key string
		in  *[]string
		out **[]string
	}{
		{"customers", ls.Customers, &out.Customers},
		{"first_names", ls.FirstNames, &out.FirstNames},
		{"last_names", ls.LastNames, &out.LastNames},
	} {
		if l.in == nil {
			continue
		}
		cleaned := make([]string, 0, len(*l.in))
		seen := make(map[string]bool, len(*l.in))
		for i, w := range *l.in {
			w = strings.TrimSpace(w)
			k := w
			if !caseSensitive {
				k = foldKey(w)
			}
			if w == "" || seen[k] {
				continue
			}
			if msg := entryProblem(w); msg != "" {
				issues = append(issues, Issue{Path: "lists_set." + l.key + "[" + strconv.Itoa(i) + "]", Message: msg})
				continue
			}
			seen[k] = true
			cleaned = append(cleaned, w)
		}
		if len(cleaned) > maxWords {
			issues = append(issues, Issue{Path: "lists_set." + l.key, Message: fmt.Sprintf("Liste hätte %d Einträge, erlaubt sind %d", len(cleaned), maxWords)})
		}
		*l.out = &cleaned
	}
	if len(issues) > 0 {
		return ListsSet{}, &ValidationError{Issues: issues}
	}
	return out, nil
}

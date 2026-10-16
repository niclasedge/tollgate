// Package jsonsplice edits selected JSON strings of an Anthropic request body
// by byte offsets, without ever re-serializing the document (ADR-0004).
//
// Scan walks the body once with a strict tokenizer and reports the byte ranges
// of the strings that stages may rewrite. Apply splices replacements into the
// original bytes: everything outside the replaced ranges stays byte-identical,
// unknown fields and content types are never visited.
package jsonsplice

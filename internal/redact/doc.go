// Package redact detects secrets in text and replaces them in a single pass.
//
// It is a Go port of the pattern mode of hook-redactor (MIT, Copyright (c)
// 2026 Niclas Edge, see third_party/NOTICE.md) following ADR-0005: all active
// detectors run on the original text, overlaps are resolved by priority,
// span length and start, and only the secret group of each match is
// replaced. Stage and Number add the numbered placeholders of ADR-0006 (#13);
// rehydration follows in #14, word lists, allowlists and custom rules in #15.
package redact

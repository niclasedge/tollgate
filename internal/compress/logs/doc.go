// Package logs holds the deterministic log strategies of the compress stage
// (ADR-0007): test_log (Headroom LogCompressor + dedupe normalizer) and
// build_log (Headroom LogTemplate + dedupe normalizer).
//
// Both strategies are pure functions of their input: no time, no randomness,
// no map iteration order in the output.
package logs

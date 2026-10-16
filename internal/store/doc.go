// Package store persists clients, configuration versions, sessions and traces
// in SQLite (modernc.org/sqlite, no cgo, WAL, one writer).
//
// Only redacted data is stored (ADR-0009): client keys appear as SHA-256
// hashes, trace hashes and diffs are derived from redacted bytes, matches
// carry offset, length, type and rule but never the value. The exported types
// deliberately have no field for request or response bodies, and the store
// validates hashes, hints and identifiers so that a plaintext value cannot be
// passed through them by accident.
package store

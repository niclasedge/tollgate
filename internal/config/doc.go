// Package config holds the bootstrap configuration of tollgate (ADR-0008,
// level 1): listen address, upstream and file paths. It is read once at start,
// never versioned and never editable from the frontend.
//
// The versioned runtime configuration (redact/compress, level 2) arrives with
// #15 and extends this package. Until then only the fields the proxy needs are
// modelled; sections owned by later issues are ignored when loading.
package config

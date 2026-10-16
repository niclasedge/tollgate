// Package sse frames Server-Sent Events and copies them event by event.
//
// The proxy (#6) uses it to pass a response stream through without buffering
// more than one event; the return path (#14) builds on the same framing. The
// reader never alters bytes: concatenating all events it returns reproduces the
// input exactly.
package sse

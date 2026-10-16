// Package pipeline is the request core of tollgate: the Stage interface, the
// per-request context, trace IDs and the runner that applies the replacement
// lists of all stages in order (ADR-0002, ADR-0004).
//
// Stages never re-serialize the body. A stage reads req.Body, returns a list of
// jsonsplice.Replacement and the runner splices them in, recording duration,
// estimated tokens, SHA-256 hashes of input and output and the configuration
// version per stage. Stage 0 (Intake) checks the client key, parses the
// request and derives the session key. There is no HTTP server in this package;
// stages operate on request bytes and headers.
package pipeline

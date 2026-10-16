// Package server is the HTTP proxy of tollgate: it runs every request through
// the pipeline (stage 0 to 3) and then forwards the result to CLIProxyAPI
// (stage 4), streaming the response back without buffering (ADR-0001).
//
// A pipeline error never leads to forwarding: the client gets the Anthropic
// error body of the stage error instead (ADR-0004, point 7). The frontend API
// and the embedded UI are added by later issues (#9, #10).
package server

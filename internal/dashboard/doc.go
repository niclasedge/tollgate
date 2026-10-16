// Package dashboard serves the read-only API of the frontend: the trace list
// and its live SSE stream (/api/traces, /api/traces/events), the cost report
// (/api/costs) and the status of tollgate and CLIProxyAPI (/api/status).
//
// The Go types in types.go are the contract with the frontend. The handlers
// carry no authentication of their own; mount them behind auth.RequireAdmin.
// Responses hold metadata only: never request content, keys or tokens.
package dashboard

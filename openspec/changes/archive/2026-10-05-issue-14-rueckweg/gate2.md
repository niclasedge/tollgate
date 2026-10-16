# Gate 2

Branch `issue-14/rueckweg`, gemeinsamer Workspace (geprüft wurde die Kombination aller
angewendeten Branches).

Rot: Mit Stubs (`Rehydrate`/`RehydrateJSON` geben die Eingabe zurück, `Holdback` puffert
nicht, `CopyRehydrated` ist `sse.CopyObserve`, `RehydrateBody` gibt den Body zurück)
schlugen in `internal/redact` fehl: `TestRehydrate`, `TestRehydrateJSON`,
`TestHoldbackSteps`, `TestHoldbackJSONMode`, `TestStreamPlaceholderAcrossDeltas`,
`TestStreamInterleavedBlocks`, `TestStreamRestAtBlockEnd`, `TestStreamRestFlushedAtEOF`,
`TestStreamUnknownEventsAndFields`, `TestStreamCRLFEvent`, `TestStreamDeterministic`,
`TestStreamHTTPChunks`, `TestRehydrateBody`, `FuzzRehydrateStream` (Seed-Korpus).

Schon mit Stubs grün (Schutztests gegen Überschreiben): `TestStreamThinkingUnchanged`,
`TestStreamEmptyMappingByteIdentical`, `TestStreamObserveSeesUpstreamEvents`,
`TestHoldbackAnySplit`.

Grün nach der Umsetzung (gemeinsamer Workspace): `go test -race ./internal/redact/...
./internal/server/...`, `go vet ./internal/redact/ ./internal/server/`, `gofmt -l` leer,
`go test -fuzz FuzzRehydrateStream -fuzztime 45s ./internal/redact/` (rund 437 000
Ausführungen, kein Fehler), `openspec validate issue-14-rueckweg --strict`.
`make test` (inkl. Frontend) lief in diesem Teil nicht; die Verdrahtung in `server.go` steht aus.

# Gate 2

Branch `issue-15/redactor-konfiguration`, gemeinsamer Workspace (Kombination aller
angewendeten Branches). Rot: mit einem Overlay, in dem `redact.Compile` nur
`New(c.Builtin)` liefert (`go test -overlay … ./internal/redact/ ./internal/redactcfg/`),
schlagen 43 Tests und Untertests fehl, darunter `TestWordListWholeWordUnicode`,
`TestAllowlistWins`, `TestParseErrors`, `TestPinnedVersionKeepsRules` und
`TestWordListLeak`. Grün nach der Umsetzung: `go test -race ./internal/redact/...
./internal/redactcfg/...` (Paritätstest gelaufen), `go vet ./...`, `make lint`,
`make test`, `go mod tidy -diff` ohne Ausgabe,
`openspec validate issue-15-redactor-konfiguration --strict`.

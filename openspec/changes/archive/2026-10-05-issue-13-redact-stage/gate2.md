# Gate 2

Branch `issue-13/redact-stage`, gemeinsamer Workspace (geprüft wurde die Kombination aller
angewendeten Branches).

Rot: Mit Stubs (`Stage.Process` und `Number` liefern nichts, `Request.Report` ohne
Wirkung im Runner, `SessionPinner.Pin` liefert 0) schlugen fehl:
`TestStageNumbering`, `TestStageLeavesProtectedBlocksByteIdentical`, `TestStageEndpoints`,
`TestStageCachePrefix`, `TestStageTraceWithoutPlaintext`, `TestStageMatchOffsetsWithEscapes`,
`TestStageDiffBounded`, `TestStageFingerprintNote`, `TestStageResolver`,
`TestStageNumberOverflowAborts`, `TestNumberIsPure` (`internal/redact`),
`TestSessionPinner`, `TestStageReportInTrace` (`internal/pipeline`) und
`TestRedactStageLeaksNothing` (`internal/server`, auch noch nach der Stufe, bis
`recordTrace` Diff und Treffer übernahm). `TestRedactSwitch` (`internal/config`) kompilierte
ohne das Feld `Redact` nicht.

Grün nach der Umsetzung: `go test -race ./...` über `make test` (Go und Frontend),
`go vet ./...`, `make lint`, `go test -fuzz FuzzStage -fuzztime 60s ./internal/redact/`
(rund 295 000 Ausführungen, kein Fehler), `openspec validate issue-13-redact-stage --strict`.

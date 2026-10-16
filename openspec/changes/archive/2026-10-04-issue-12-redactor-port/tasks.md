# Tasks

## 1. Tests zuerst

- [x] 1.1 Korpus unter `internal/redact/testdata/corpus/` aus den Rust-Tests übernehmen (inkl. red_team §7, §9).
- [x] 1.2 `corpus_test.go`, `redact_test.go` (Tabellen, Schalter, Auflösung, Determinismus), `fuzz_test.go`, `parity_test.go`; rot sehen.

## 2. Umsetzung

- [x] 2.1 `rules.go`: Options, Regeltabelle, Typen, Prioritäten.
- [x] 2.2 `falsepositive.go`: Port von `is_false_positive` und Helfern.
- [x] 2.3 `redact.go`: Find, Auflösung, Replace, Redact.

## 3. Abschluss

- [x] 3.1 Paritätstest läuft lokal grün; Abweichungen in ADR-0005 nachgetragen.
- [x] 3.2 Herkunft in `third_party/NOTICE.md`.
- [x] 3.3 `go test -race ./internal/redact/...`, `go vet`, `make lint`, `make test`; `openspec validate issue-12-redactor-port --strict`.

# Tasks

## 1. Tests zuerst

- [x] 1.1 `TestClientSecretBounds` (doppelte und einfache Anführungszeichen, JSON kompakt, maskiertes `\"`, maskierter Backslash, kurzer Wert, offenes Anführungszeichen, unquotiert, leerer Wert).
- [x] 1.2 Korpus: `patterns/client_secret_snake_case` (want mit schließendem `"`), neu `tollgate/client_secret_json`.
- [x] 1.3 `prefilter_test.go`: `TestPrefilterKeepsCorpusFindings`, `TestPrefilterSkipsRules`, `TestPrefilterCaseFolding`, `TestFoldOrbitsToASCII`, `FuzzPrefilter`; Stubs (`need`, `fold`, `trim`, `noPrefilter`, `prefilter.pass` liefert immer wahr).
- [x] 1.4 `prefilter_bench_test.go`: `BenchmarkFind`, Vorher-Zahlen erfasst.
- [x] 1.5 Rot sehen (Gate 2).

## 2. Umsetzung

- [x] 2.1 `trimQuoted` und Aufruf in `Find`; `patterns/ps_secret_client_secret` an die neue Spanne angepasst.
- [x] 2.2 Literale je Regel, `prefilter`, `foldASCII`, Aufruf in `Find`.
- [x] 2.3 Benchmark nachher, Zahlen in `design.md`.
- [x] 2.4 ADR-0005: Nachtrag #84.

## 3. Abschluss

- [x] 3.1 `go test -race ./internal/redact/`, Paritätstest mit `TOLLGATE_REDACTOR_REPO`, Fuzz `FuzzPrefilter` und `FuzzRedact`, `make lint`, `make test`.
- [x] 3.2 `openspec validate issue-84-client-secret-vorfilter --strict`.

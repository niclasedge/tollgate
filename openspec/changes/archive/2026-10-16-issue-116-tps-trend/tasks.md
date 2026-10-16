# Tasks

Umsetzung erst, wenn #87 gemergt ist; vorher die Migrationsnummer prüfen.

## 1. Statistik (`internal/perf`)

- [x] 1.1 Tests zuerst: `tps_test.go` und `influence_test.go` tabellengetrieben (`TPSMilli`, Cache-Klassen, Urteil inkl. Rückfall und Mindestausgabe, Matrix, Kernaussagen, permutierte Eingabe); rot sehen.
- [x] 1.2 `tps.go`: `TPSMilli`, `JudgeTPS`; `influence.go`: `CacheClass`, `Matrix`, `Insights`.

## 2. Store

- [x] 2.1 Tests zuerst: Migration auf DB mit #87-Schema, `PutTPS`/Lesen, Kaskade, Vergleichsabfrage (SSE, TTFT vorhanden, Ausgabe ≥ 64, Fenster, Limit 500); rot sehen.
- [x] 2.2 Migration `00NN_trace_tps.sql`, `tps.go`, Lesen in `traces.go`.

## 3. Proxy

- [x] 3.1 Tests zuerst (`internal/server`): 20 schnelle Streams, dann ein langsamer → `tps.verdict = langsamer` im Event; Store-Fehler blockiert nicht; rot sehen.
- [x] 3.2 `finishTrace`: Urteil Tokens/s nach dem Urteil aus #87 berechnen und speichern.

## 4. Dashboard-API

- [x] 4.1 Tests zuerst: `tps` in `TraceSummary` (auch `null`), `influence` mit 12 Zellen, Kernaussagen, Trendfelder in `days`; rot sehen.
- [x] 4.2 `types.go`, `convert.go`, `performance.go`; `make generate` (tygo).

## 5. Frontend

- [x] 5.1 Tests zuerst (Vitest): Spalte „t/s“ je Urteil, Tooltip, „–“ bei laufend/Nicht-Stream, Detailzeile, Matrix mit leeren Zellen, Kernaussagen, Trendlinien; rot sehen.
- [x] 5.2 Spalte (Helfer `tpsCell` in `perf.ts` statt eigener `TpsCell.svelte`) und Detailzeile in `Live.svelte`.
- [x] 5.3 `InfluenceMatrix.svelte`, Kernaussagen und Trendlinien in `Performance.svelte`.
- [x] 5.4 Nachbesserung nach Rückmeldung (2026-10-05): `TpsChart.svelte` über der Modelltabelle, Median Tokens/s je Tag mit einer Linie je Modell (`tpsByDay` in `perf.ts`), Tests zuerst.

## 6. Doku und Abschluss

- [x] 6.1 `docs/ARCHITECTURE.md`: Abschnitt Performance um Tokens/s-Urteil und Einfluss-Matrix ergänzen.
- [x] 6.2 Abnahme: Live zeigt Tokens/s mit Vergleich; Performance zeigt Matrix, Kernaussagen und Trend.
- [x] 6.3 `go test -race ./...`, `go vet ./...`, `make lint`, `make test`.
- [x] 6.4 `openspec validate issue-116-tps-trend --strict`.

## 7. Korrektur Tokens/s-Basis

- [x] 7.1 Generierungsstart `gen_start_us` (Flush von `message_start`, Migration 0008) statt TTFT als Basis für Tokens/s; alte Urteile verworfen. Tests: Server (verdecktes Thinking), Store, Dashboard.

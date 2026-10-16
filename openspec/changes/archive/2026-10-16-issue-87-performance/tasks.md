# Tasks

Umsetzung erst nach Abschluss der laufenden Issues; vorher prüfen, dass #13 und #14
geschlossen sind und die Migrationsnummer noch frei ist.

## 1. Statistik (`internal/perf`)

- [x] 1.1 Tests zuerst: `perf_test.go` tabellengetrieben für Nearest-Rank (n = 1, 2, 20, 21, gleiche Werte), Klassengrenzen Eingabe/Ausgabe, Rückfall der Dauer-Klasse, Schwelle (jede der drei Bedingungen einzeln verletzt), `unbekannt` bei 19 Werten, permutierte Eingabe, Tokens/s- und Eigenanteil-Grenzfälle; rot sehen.
- [x] 1.2 `perf.go`: `Percentile`, `InputClass`, `OutputClass`, `Judge`, `OutputTPS`, `TollgateMicros`, Erklärungsdaten.

## 2. Store

- [x] 2.1 Tests zuerst: Migration auf bestehender DB (alte Traces mit `NULL`), neue Spalten über `FinishTrace` schreiben/lesen, Wertebereiche, `trace_perf` validiert und kaskadiert, Vergleichsabfrage (Fenster, Filter, Limit 500, Reihenfolge), reservierter Client (Name/ID abgelehnt, keine Rotation, kein Schlüssel führt zu `speedtest`), Leck-Test erweitert; rot sehen.
- [x] 2.2 Migration `00NN_trace_performance.sql` (Spalten, Index, `trace_perf`, reservierter Client).
- [x] 2.3 `traces.go`/`perf.go`/`clients.go`: `TraceResult`-Felder, `PutPerf`, `PerfSamples`, Perf beim Lesen der Traces, Schutz des reservierten Clients.

## 3. Proxy und Pipeline

- [x] 3.1 Tests zuerst (`internal/server`): `httptest`-SSE-Upstream mit 300 ms Verzögerung vor dem ersten `content_block_delta` → `ttft_us` im Bereich und Stream byte-gleich; JSON-Upstream mit verzögerten Headern; 429 → `NULL`; Stufen-Bytes im Trace; 20 schnelle Anfragen, dann eine verzögerte → Live-Event `langsam`; Store-Fehler beim Urteil blockiert nicht; Rückweg-Messwerte (mit #14); rot sehen.
- [x] 3.2 Tests zuerst (`internal/pipeline`, `internal/server`): interne Identität aus dem Kontext, Header/Body können sie nicht nachahmen, nur `redact` überspringbar; `Probe` ohne/mit Redact erzeugt Trace mit/ohne Stufe `redact`; Speedtest-Text nicht in DB und Log; rot sehen.
- [x] 3.3 `server.forward`: TTFT messen (SSE nach Flush des ersten `content_block_delta`, JSON bei Headern), Rückweg-Zähler aus #14 übernehmen; `recordTrace`: `BytesIn/BytesOut`; `finishTrace`: Werte schreiben, Urteil berechnen und speichern, dann Event.
- [x] 3.4 `pipeline`: `WithInternalClient`, `Intake.Authenticate` nimmt die interne Identität, Runner überspringt `redact` nur dafür; `server.Probe`.

## 4. Dashboard-API

- [x] 4.1 Tests zuerst (`internal/dashboard`, `internal/auth`): `/api/performance` (Verteilungen, Sortierung, Tage, `days` ungültig 400, Speedtest ausgeschlossen und unter `speedtests`, Redactor `null`/gefüllt, Admin-Token Pflicht); `TraceSummary` mit `ttft_ms`, `output_tps`, `tollgate_ms`, `perf` (auch `null` für alte Traces); `/api/speedtest` ohne Bestätigung 400 ohne Upstream-Aufruf, 0/5 Modelle 400, parallel 409, Erfolg mit zwei Ergebnissen, Redact aus; `/api/clients` ohne internen Client, Rotation 404; rot sehen.
- [x] 4.2 `types.go`, `convert.go`, `performance.go` (inkl. Speedtest-Handler, statt eigener `speedtest.go`), Routen in `handler.go`; Verdrahtung in `cmd/tollgate/main.go`; Client-API-Filter in `internal/auth/adminapi.go`.
- [x] 4.3 `make generate` (tygo) für `web/src/lib/types/dashboard.ts`.

## 5. Frontend

- [x] 5.1 Tests zuerst (Vitest): roter Punkt und Erklärtext bei `perf.slow` in Live und Startseite, laufend/Fehler behalten ihren Punkt, „noch zu wenig Vergleichswerte“; Performance-Tabelle mit min/median/p90/max und Zeitraum-Wechsel; Redactor-Karte mit Hinweis und „Noch keine Daten“; Speedtest mit Mock: Abbrechen sendet nichts, Bestätigen sendet `confirm: true`, Ergebnis mit üblichem Median; Sidebar-Eintrag und Kürzel `8`; rot sehen.
- [x] 5.2 `StatusDot` Zustand `slow`, Format-Helfer für Dauer/Erklärung, `api.ts`.
- [x] 5.3 `Performance.svelte` (Tabelle, Verlauf, Redactor-Karte, Speedtest-Karte mit Dialog), Route und Kürzel in `router.svelte.ts`, `App.svelte`.
- [x] 5.4 `Live.svelte` (Liste, Inline-Detail) und `Home.svelte` (letzte Anfragen) mit Perf-Punkt und Detailwerten.

## 6. Kommandozeile und Doku

- [x] 6.1 `deploy/setup.sh speedtest` (Bestätigung, `--yes`, Admin-Token über stdin) und Rezept `just speedtest`; Shell-Test oder Smoke-Prüfung mit Mock-API.
- [x] 6.2 `docs/ARCHITECTURE.md`: Abschnitt Performance (TTFT-Definition, Erwartungswert-Modell, Speedtest-Pfad).

## 7. Abschluss

- [x] 7.1 Abnahme: Performance-Ansicht zeigt je Modell min/median/p90/max; künstlich verzögerte Anfrage erscheint rot mit Erklärung; Speedtest liefert Werte mit und ohne Redactor.
- [x] 7.2 `go test -race ./...`, `go vet ./...`, `make lint`, `make test`.
- [x] 7.3 `openspec validate issue-87-performance --strict`.

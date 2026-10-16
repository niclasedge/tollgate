# Proposal

Issue: [#87 Performance: Antwortzeiten je Modell, Ausreißer rot, Redactor-Kosten, Speedtest](https://github.com/niclasedge/tollgate/issues/87)

> **Umsetzung erst nach Abschluss der laufenden Issues** (Wunsch Niclas). Der Change ist
> vorbereitet, aber noch nicht begonnen. Abhängigkeiten: #13 (Redact-Stufe), #14 (Rückweg),
> #76 (Startseite, gemergt). Welche Teile wovon abhängen, steht in `design.md`.

## Why

tollgate misst heute je Anfrage nur die Gesamtdauer (`traces.duration_us`) und die Dauer der
Stufen vor dem Weiterleiten. Ob Claude gerade langsam ist, ob tollgate selbst bremst und was
der Redactor kostet, lässt sich daraus nicht ablesen. Es fehlt die Zeit bis zum ersten Token,
ein Vergleich mit „gewöhnlichen“ Antwortzeiten und ein Weg, die Geschwindigkeit gezielt zu
prüfen.

## What Changes

- **Messwerte je Anfrage:** Zeit bis zum ersten Token (TTFT), Rückweg-Dauer und Zahl der
  Rückübersetzungen werden im Trace gespeichert; Ausgabe-Tokens pro Sekunde und der
  tollgate-Eigenanteil werden daraus berechnet. Nur Zahlen, keine Inhalte.
- **Erwartungswert und Urteil „langsam“:** Je Modell und Größenklasse wird aus den eigenen
  Daten der letzten 14 Tage vor der Anfrage Median und p90 gebildet. Liegt die Anfrage
  deutlich darüber, gilt sie als langsam. Das Urteil wird beim Abschluss einmal berechnet und
  gespeichert, mit Vergleichswerten für eine erklärbare Aussage.
- **Store:** neue Migration mit den Spalten `ttft_us`, `return_us`, `return_replacements` in
  `traces`, neuer Tabelle `trace_perf` für das Urteil und einem Index für die Vergleichsabfrage.
- **Dashboard-API:** `GET /api/performance?days=N` (Statistik je Modell, Verlauf, Anteil
  langsamer Anfragen, Redactor-Kosten, letzte Speedtests), `POST /api/speedtest`, neue Felder
  in `TraceSummary` (`ttft_ms`, `output_tps`, `tollgate_ms`, `perf`).
- **Frontend:** neuer Bereich „Performance“ unter „Beobachten“, roter Statuspunkt mit
  Erklärung in Live (Liste und Detail) und auf der Startseite, Speedtest mit Bestätigung.
- **Speedtest:** feste Testanfrage je Modell über die volle Pipeline mit einem internen
  Client, einmal ohne und einmal mit Redact-Stufe; dazu `just speedtest`.

## Capabilities

### New Capabilities

- `performance`: Messwerte, Erwartungswert-Modell, Urteil „langsam“, Redactor-Kosten,
  Speedtest.

### Modified Capabilities

- `store`: Performance-Spalten, Tabelle `trace_perf`, interner Speedtest-Client.
- `client-auth`: interner Speedtest-Client in der Client-API verborgen.
- `proxy`: TTFT-Messung und Rückweg-Messwerte beim Weiterleiten.
- `dashboard-api`: Performance-Bericht, Speedtest-Endpunkt, Performance-Felder in Traces.
- `frontend-views`: Performance-Ansicht, roter Statuspunkt, Speedtest.
- `frontend-shell`: Eintrag „Performance“ in der Gruppe Beobachten.

## Impact

Branch `issue-87/performance`, gemeinsamer Workspace. Voraussichtlich berührte Dateien:

- Neu: `internal/perf/` (Statistik, Größenklassen, Urteil, Speedtest-Anfrage),
  `internal/store/migrations/00NN_trace_performance.sql` (nächste freie Nummer, Stand heute
  `0006`), `internal/store/perf.go`, `internal/dashboard/performance.go`,
  `internal/dashboard/speedtest.go`, `web/src/lib/views/Performance.svelte`,
  `web/src/lib/components/PerfDot.svelte`, zugehörige Tests.
- Eigene Hunks in: `internal/store/traces.go` (`TraceResult`, Lesen der neuen Spalten),
  `internal/store/clients.go` (reservierter Client), `internal/auth/adminapi.go` (Liste
  filtern, Rotation verweigern), `internal/server/server.go` (TTFT,
  Rückweg-Messwerte, `BytesIn/BytesOut` der Stufen, interner Aufruf),
  `internal/pipeline/intake.go` (interne Identität), `internal/pipeline/pipeline.go`
  (Stufe überspringen), `internal/dashboard/{types,convert,handler}.go`,
  `cmd/tollgate/main.go` (Verdrahtung), `tygo.yaml`, `web/src/lib/types/dashboard.ts`
  (generiert), `web/src/lib/api.ts`, `web/src/lib/router.svelte.ts`, `web/src/App.svelte`,
  `web/src/lib/views/{Live,Home}.svelte`, `Justfile`, `deploy/setup.sh` (Rezept
  `speedtest`), `docs/ARCHITECTURE.md` (Abschnitt Performance), diese Change-Artefakte.

## Was ausdrücklich nicht

- Keine Inhalte in Store, Log oder API: weder Prompt noch Antwort noch Speedtest-Text.
- Kein OpenTelemetry (#24), kein Budget-Alarm (#23), keine Benachrichtigung bei Langsamkeit.
- Keine Anfragen-Timeline mit Stufen-Diffs (#16); die Performance-Werte erscheinen im
  Inline-Detail der Live-Liste, #16 übernimmt die Komponente später.
- Kein automatischer oder periodischer Speedtest; er läuft nur auf ausdrücklichen Auftrag.
- Kein nachträgliches Urteil für Traces vor der Migration (kein Backfill).
- Keine Änderung an Redact-Logik, Platzhaltern oder Rückweg-Algorithmus (#13/#14).

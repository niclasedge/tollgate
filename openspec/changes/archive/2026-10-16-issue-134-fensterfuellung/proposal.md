# Proposal

Issue: [#134 Kosten & Performance: schnellstes Füllen des 5-h-Fensters, Kennzahlen als Kacheln](https://github.com/niclasedge/tollgate/issues/134)

## Why

Wie schnell ein Abo sein 5-h-Fenster leerläuft, ist bisher nicht sichtbar. Die Auslastung kennt tollgate nur als letzten Stand je Account, nicht im Verlauf. Die sechs Kennzahlen unter Kosten & Performance stehen gleichrangig nebeneinander und sind schwer zu überfliegen.

## What Changes

- Jeder `/v1/messages`-Trace speichert die 5-h-Auslastung (Promille) und den Reset-Zeitpunkt aus den Antwort-Headern (Migration 0009). Ältere Traces bleiben ohne Wert.
- `GET /api/performance` liefert `fastest_fill`: das Fenster (Account + Reset) mit der kürzesten hochgerechneten Zeit bis 100 %, dazu Anstieg, beobachtete Dauer, Anfragen und Ø Anfragen/min. Nur Fenster mit mindestens 300 Promille Anstieg zählen.
- Frontend: Kachel „Schnellstes 5-h-Fenster“; die bisherigen Kennzahlen werden zu Kacheln mit großer Zahl und kleiner Zeile darunter zusammengefasst.

## Capabilities

### New Capabilities

- keine

### Modified Capabilities

- `store`: 5-h-Auslastung am Trace.
- `dashboard-api`: Performance-Bericht mit `fastest_fill`.
- `frontend-views`: Kennzahlen-Kacheln unter Kosten & Performance.

## Impact

- `internal/store/migrations/0009_trace_five_hour.sql`, `internal/store/traces.go`, `internal/store/fill.go` (neu)
- `internal/server/server.go` (`forward`, `finishTrace`)
- `internal/dashboard/fill.go` (neu), `performance.go`, `types.go`, generierte TS-Typen
- `web/src/lib/views/Costs.svelte`, `web/src/lib/fill.ts` (neu), Tests

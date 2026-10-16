# Proposal

Issue: [#136 Kosten & Performance: Einschätzung 5-h-Fenster je Wochenlimit, Bedarf und Kapazität](https://github.com/niclasedge/tollgate/issues/136)

## Why

Ob die vorhandenen Abos für eine Woche reichen, lässt sich bisher nur schätzen. tollgate sieht bei jeder Antwort die 5-h- und die 7-Tage-Auslastung, speichert aber seit #134 nur die 5-h-Werte je Anfrage.

## What Changes

- Jeder `/v1/messages`-Trace speichert auch die 7-Tage-Auslastung (Promille) und deren Reset (Migration 0010).
- `GET /api/performance` liefert `capacity`: Ø volle 5-h-Fenster je Wochenlimit, gefüllte 5-h-Fenster pro Tag, Preis eines vollen Fensters, Zahl der Accounts und noch freie Fenster dieser Woche. Fenster je Wochenlimit = Kosten eines Accounts seit Beginn seines Wochenfensters in Preisen eines vollen 5-h-Fensters, bezogen auf seinen Wochenverbrauch. Das geht sofort aus der Kostenhistorie; die 7-Tage-Werte am Trace bleiben als Historie zur Kontrolle der Schätzung.
- Kosten & Performance zeigt eine Einschätzung: Fortschrittsbalken Bedarf pro Woche gegen Kapazität aller Accounts, Urteil „reicht“, „knapp“ (ab 80 %) oder „reicht nicht“ (über 100 %), dazu Kennzahlen mit großer Zahl und kleiner Zeile, auch „noch frei diese Woche“ aus der aktuellen 7-Tage-Auslastung.

## Capabilities

### Modified Capabilities

- `store`: 7-Tage-Auslastung am Trace.
- `dashboard-api`: Performance-Bericht mit `capacity`.
- `frontend-views`: Einschätzung unter Kosten & Performance.

## Impact

- `internal/store/migrations/0010_trace_seven_day.sql`, `internal/store/traces.go`, `internal/store/fill.go`
- `internal/server/fivehour.go`, `internal/server/server.go` (`forward`, `finishTrace`)
- `internal/accounts/usage.go` (`CachedUsage`: Verbrauch lesen ohne Abfrage)
- `internal/dashboard/capacity.go` (neu), `performance.go`, `types.go`, generierte TS-Typen
- `web/src/lib/capacity.ts` (neu), `web/src/lib/components/CapacityCard.svelte` (neu), `web/src/lib/views/Costs.svelte`, Tests

## Why
Auf der Übersicht fehlt ein Blick darauf, ob gerade ungewöhnlich viel über tollgate läuft. Gewünscht ist die Zahl der Anfragen der letzten 5 Minuten und ob sie über dem Durchschnitt der 10 Fünf-Minuten-Fenster davor liegt. Issue: https://github.com/niclasedge/tollgate/issues/132.

## What Changes
- Store: `CountTraces(from, to)` zählt Traces mit `from <= started_at < to` über den vorhandenen Index `traces_started_at`.
- Neuer Endpunkt `GET /api/calls` hinter dem Admin-Token: Anfragen im aktuellen Fenster [jetzt−5 min, jetzt), Summe und Ø der 10 Fenster [jetzt−55 min, jetzt−5 min) (leere Fenster zählen als 0) und die Einordnung `above`, `below` oder `equal`. Calls sind alle Traces, gleich welcher Endpunkt oder Status. Keine Inhalte, kein Logging von Inhalten.
- Übersicht: Abschnitt „Letzte 5 Minuten“ mit Zahl, Ø der 10 Fenster davor und rundem Statusindikator mit „über Ø“, „unter Ø“ oder „wie Ø“. Er lädt mit den übrigen Daten der Übersicht neu (alle 30 s und per „Aktualisieren“).

## Capabilities
### New Capabilities
### Modified Capabilities
- `dashboard-api`: Endpunkt `/api/calls`.
- `frontend-views`: Abschnitt „Letzte 5 Minuten“ auf der Übersicht.

## Impact
Branch `issue-132/calls-5min`, gemeinsamer Workspace. Dateien:
- `internal/store/calls.go`, `internal/store/calls_test.go`
- `internal/dashboard/calls.go`, `internal/dashboard/calls_test.go`, `internal/dashboard/types.go` (Typ `CallRate`), `internal/dashboard/handler.go` (Store-Interface, Route)
- `web/src/lib/types/dashboard.ts` (generiert)
- `web/src/lib/api.ts` (nur Methode `calls`), `web/src/lib/components/CallRate.svelte`, `web/src/lib/views/Home.svelte` (nur Import und Abschnitt), `web/src/calls.test.ts`
- diese Artefakte

Keine Migration, keine neue Abhängigkeit.

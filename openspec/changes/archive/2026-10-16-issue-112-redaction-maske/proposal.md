## Why
In Live und im Panel „Letzte Redactions“ ist heute nur Typ, Regel und Platzhalter zu sehen. Wer prüfen will, **welcher** Wert ersetzt wurde, hat keinen Anhaltspunkt, ohne dass Klartext in DB, Log oder Frontend landen darf (Wunsch aus der Phase-2-Abnahme #17). Issue: https://github.com/niclasedge/tollgate/issues/112.

## What Changes
- Neue reine Funktion `redact.Mask(rule, typ, value)` berechnet je Treffer eine deterministische Maske, z. B. `n•••@g•••.com` oder `sk-ant-api03-a••• (108)`. Sie läuft in der Redact-Stufe, dem einzigen Ort mit Klartext.
- `pipeline.Match`, `store.Match` und `dashboard.Match`/`dashboard.Redaction` bekommen das Feld `mask`. Gespeichert und übertragen wird nur die Maske; der Store lehnt Masken ohne `•`, mit Steuerzeichen oder über 64 Bytes ab.
- `GET /api/traces/{id}` und `GET /api/redactions` liefern die Maske je Treffer; ältere Traces liefern `""`.
- Frontend: Die Redact-Stufe der Timeline listet je Platzhalter Typ, Platzhalter und Maske; „Letzte Redactions“ zeigt eine Spalte Maske.
- Bodies an Upstream, Platzhalter, Diff und Cache-Präfixe bleiben unverändert; die Maske ist nur Metadatum des Traces.

## Capabilities
### New Capabilities
Keine.
### Modified Capabilities
- `redaction`: Maske je Treffer.
- `store`: Maske in gespeicherten Treffern.
- `dashboard-api`: Maske in Trace-Detail und letzten Redactions.
- `frontend-views`: Anzeige der Maske in Timeline und Übersicht.

## Impact
Branch `issue-112/redaction-maske`, gemeinsamer Workspace. Dateien:
- `internal/redact/mask.go`, `internal/redact/mask_test.go`, `internal/redact/stage.go`, `internal/redact/stage_test.go`
- `internal/pipeline/pipeline.go`
- `internal/server/server.go`
- `internal/store/traces.go`, `internal/store/redactions.go`, `internal/store/errors.go`, `internal/store/redactions_test.go`, `internal/store/store_test.go` (Feld-Allowlist um `Mask` ergänzt)
- `internal/dashboard/types.go`, `internal/dashboard/detail.go`, `internal/dashboard/detail_test.go`
- `cmd/tollgate/acceptance_test.go` (Leck-Test erweitert)
- `web/src/lib/types/dashboard.ts` (generiert), `web/src/lib/components/Timeline.svelte`, `web/src/lib/views/Home.svelte`, `web/src/test-helpers.ts`, `web/src/timeline.test.ts`, `web/src/home.test.ts`
- diese Artefakte

Keine Migration: Treffer liegen als JSON in `trace_stages.matches`.

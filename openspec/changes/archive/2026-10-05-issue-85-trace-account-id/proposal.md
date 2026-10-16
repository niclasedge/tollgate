## Why
Nutzerauftrag: Account-ID zusätzlich je Anfrage sichern, damit Tokens und Preisvergleich dem verwendeten Account zugeordnet werden können. Issue: https://github.com/niclasedge/tollgate/issues/85. Umsetzung ausdrücklich beauftragt.
## What Changes
Account-ID einmalig nach Eintritt in die Request-/Wechsel-Sperre aus dem tatsächlich aktiven CPA-Account bestimmen und mit dem Trace speichern. Zuordnung in Listen, Details und SSE erhalten; Kostenaggregation zusätzlich nach Account trennen. Alte Traces bleiben unzugeordnet, historische IDs bleiben auch nach Account-Löschung erhalten. Live zeigt Kurz-ID und Detail die volle ID; Kosten zeigt Pro-Account-Auswertung. Preise bleiben USD-Listenpreisvergleich.
## Capabilities
### New Capabilities
- `trace-account-attribution`: unveränderliche Account-Zuordnung je Anfrage.
## Impact
Branch `issue-85/trace-account-id`, gemeinsamer Workspace. Dateien: `internal/accounts/{manager,trace_account_test}.go`, `internal/server/{server,trace_account_test}.go`, `internal/store/{traces,trace_account_test,store_test}.go`, `internal/store/migrations/0005_trace_account_id.sql`, `internal/dashboard/{types,convert,handler,trace_account_test}.go`, `cmd/tollgate/main.go`, `web/src/lib/types/dashboard.ts` (generiert), `web/src/lib/views/{Live,Costs}.svelte`, `web/src/trace-account.test.ts`, `docs/DEPLOY.md`, diese Change-Artefakte. Keine Payload-/Headertransformation oder Preisänderung.

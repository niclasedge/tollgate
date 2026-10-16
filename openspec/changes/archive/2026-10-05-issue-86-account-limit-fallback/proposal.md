## Why
Gemeldeter Bug: Nach einem Wechsel zeigt die Account-Karte HTTP 429 und unbekannte Limits, während die Übersicht gültige Antwort-Limits kennt. Issue: https://github.com/niclasedge/tollgate/issues/86. Nutzerauftrag zur Korrektur während #85.
## What Changes
Validierte Limits aus echten Antworten und Beispielaufrufen mit der unveränderlichen Request-Account-ID verbinden, neuesten Messstand je Account speichern und in Usage-Karten nutzen. Eine gedrosselte separate API darf neuere gültige Antwortwerte nicht verdrängen. Ein kurzlebiger, pro Manager einzelner Hintergrundarbeiter speichert zusammengefasste Messstände; Proxy-Headerverarbeitung blockiert nicht auf DB/HTTP.
## Capabilities
### New Capabilities
- `account-response-limits`: Antwort-Limits als Account-bezogener Messstand.
## Impact
Branch `issue-86/account-limit-fallback`, gemeinsamer Workspace, Integration mit #85. Dateien: `internal/accounts/{manager,usage,types,response_limits_test}.go`, `internal/upstream/accounts.go`, `internal/server/{server,trace_account_test}.go`, `cmd/tollgate/main.go`, `web/src/lib/views/AccountManager.svelte`, `web/src/lib/components/AccountUsage.svelte`, `web/src/lib/types/accounts.ts` (generiert), `web/src/account-usage.test.ts`, `docs/DEPLOY.md`, diese Change-Artefakte. Keine Änderungen an Client-Payload, CPA oder Preisen.

## Why
Nutzerauftrag: Wochenstatus pro Account, verbleibendes Budget, Zeitfortschritt und Reset auf einen Blick. Issue: https://github.com/niclasedge/tollgate/issues/78. Explizite Freigabe zur Umsetzung am 2026-10-04.
## What Changes
Separater OAuth-Usage-Read für aktive und inaktive Accounts ohne Modellaufruf oder Wechsel. Fünf Minuten Cache, gespeicherter letzter Messstand, sichere Fehlermeldungen. Account-Karten zeigen 5-Stunden- und Wochenverbrauch sowie die Zeitposition innerhalb der vom Reset rückwärts gerechneten sieben Tage. Unbekannte und veraltete Werte werden nicht als verfügbares Budget dargestellt.
## Capabilities
### New Capabilities
- `account-weekly-usage`: Account-bezogene Usage-Abfrage und Wochenanzeige.
## Impact
Branch: `issue-78/account-wochenlimits`, gemeinsamer Workspace auf Basis der vorhandenen Account-Verwaltung. Dateien: `internal/accounts/{types,manager,usage,usage_test}.go`, `internal/upstream/{usage,usage_test}.go`, `internal/store/accounts.go`, `internal/store/migrations/0004_account_usage.sql`, `internal/dashboard/accounts.go`, `web/src/lib/api.ts`, `web/src/lib/types/accounts.ts` (generiert), `web/src/lib/components/AccountUsage.svelte`, `web/src/lib/views/AccountManager.svelte`, `web/src/lib/views/Accounts.svelte`, `web/src/accounts.test.ts`, `web/src/account-usage.test.ts`, `docs/DEPLOY.md`, diese Change-Artefakte. Keine Änderung an CPA, Client, Testbefehlen oder CI.

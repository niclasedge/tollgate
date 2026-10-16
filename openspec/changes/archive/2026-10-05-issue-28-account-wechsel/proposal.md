## Why
Der Nutzer will als erstes Accounts vollständig im Frontend hinzufügen und manuell wechseln, einschließlich Login-URL und Beispielaufruf. Issue: https://github.com/niclasedge/tollgate/issues/28. Backend-Abhängigkeiten #3 und #4 sind geschlossen. Expliziter Nutzerauftrag zur Umsetzung und zu `just setup` am 2026-10-04.

## What Changes
Verschlüsselter SQLite-Tresor, PKCE-Login ohne zweite aktive Datei, Import, Liste, Entfernen inaktiver Accounts, sicherer manueller Wechsel mit Rücksicherung und Beispielaufruf. Der bestehende Account wird übernommen. Minimaler Frontend-Ablauf gehört auf ausdrücklichen Nutzerauftrag zu diesem Change; der umfassende Lizenz-/Übersichts-Ausbau aus #29/#76 wird nicht begonnen.

## Capabilities
### New Capabilities
- `account-management`: Tresor, Login, Wechsel und Prüfung.

## Impact
Branch: `issue-28/account-wechsel`. Gemeinsamer Workspace; fremder Change #72 wird nicht verändert. Berührte Dateien: `internal/accounts/*.go` (einschließlich `integration_test.go`), `internal/cryptobox/*.go`, `internal/store/accounts.go`, `internal/store/migrations/0003_accounts.sql`, `internal/upstream/accounts.go`, `internal/upstream/accounts_test.go`, `internal/config/config.go`, `internal/server/server.go`, `internal/dashboard/handler.go`, `internal/dashboard/accounts.go`, `internal/dashboard/types.go`, `internal/meter/limits.go`, `cmd/tollgate/main.go`, `web/src/lib/api.ts`, `web/src/lib/types/dashboard.ts` (generiert), `web/src/lib/views/AccountManager.svelte`, `web/src/lib/views/Accounts.svelte`, `web/src/accounts.test.ts`, `deploy/tollgate/entrypoint.sh`, `deploy/tollgate/Dockerfile`, `tygo.yaml`, `web/src/lib/types/accounts.ts` (generiert), `docs/adr/0015-account-tresor-und-wechsel.md`, `docs/ARCHITECTURE.md`, `docs/DEPLOY.md`, diese Change-Artefakte. Tests: `internal/accounts/manager_test.go`, `internal/accounts/oauth_test.go`, `internal/accounts/gate_test.go`, `internal/cryptobox/box_test.go`, `web/src/accounts.test.ts`.

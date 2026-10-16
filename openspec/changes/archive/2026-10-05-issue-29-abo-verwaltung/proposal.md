## Why
Issue: https://github.com/niclasedge/tollgate/issues/29. Abhängigkeiten #10, #28 und #76 sind geschlossen. Die Account-Verwaltung aus #28, #77, #78 und #88 deckt bereits Liste, Import, Login per Callback-URL, Wechsel mit Inline-Bestätigung, Cache-Hinweis, Fortschritt, Rollback-Meldung und die Wochenkarte ab. Es fehlen laut Issue noch: Token-Gültigkeit bzw. „neu einloggen nötig“, Plan und Abo-Status, der 5-Stunden-Wert als Balken, „Stand vom …“ für inaktive Abos, dieselbe Karte je Abo auf der Startseite und Escape zum Abbrechen der Inline-Bestätigung.

## What Changes
- Backend liest beim ohnehin gedrosselten Usage-Read zusätzlich `api/oauth/profile` mit demselben, nur im RAM gehaltenen Token. Gespeichert werden nur validierte feste Bezeichner (Organisationstyp, Rate-Limit-Stufe, Abo-Status), kein Body, keine Namen, keine Tokens.
- Ein Token-Zustand `valid`, `expired`, `relogin` oder `unknown` wird aus beiden Reads und dem Ablaufdatum der Token-Datei abgeleitet und mit Prüfzeit im vorhandenen Usage-Messstand persistiert (keine Migration).
- Neue Karte `AccountCard` je Abo: Name, E-Mail, runder Aktiv-Indikator, Lizenzzeile (Token, Plan, Status, „Stand vom …“) und Limits (5 Stunden und Woche je als dezenter Balken mit Reset). Accounts und Startseite nutzen dieselbe Karte.
- Inaktive Abos mit `relogin` bieten „Neu anmelden“ an (bestehender Login-Ablauf, aktualisiert das gespeicherte Abo).
- Escape bricht die Inline-Bestätigung ab und setzt den Fokus zurück.

## Capabilities
### New Capabilities
- `account-license`: Lizenzstatus je Abo (Token, Plan, Status) ohne Wechsel.
### Modified Capabilities
- `frontend-views`: Startseite zeigt mit Account-Verwaltung eine Karte je Abo.

## Was ausdrücklich nicht
- Kein Refresh inaktiver Tokens durch tollgate, kein Wechsel, keine Änderung an CPA, auth-dir, Testbefehl oder CI.
- Neu-Anmelden des gerade aktiven Abos bleibt blockiert (Verhalten aus #28).

## Impact
Branch `issue-29/abo-frontend`, gemeinsamer Workspace. Paralleler Change #75 (deploy/, docker-compose.yml) wird nicht berührt. Dateien: `internal/upstream/profile.go`, `internal/upstream/profile_test.go`, `internal/accounts/types.go`, `internal/accounts/usage.go`, `internal/accounts/license_test.go`, `web/src/lib/types/accounts.ts` (generiert), `web/src/lib/license.ts`, `web/src/lib/components/AccountCard.svelte`, `web/src/lib/components/AccountUsage.svelte`, `web/src/lib/views/AccountManager.svelte`, `web/src/lib/views/Home.svelte`, `web/src/license.test.ts`, `web/src/account-usage.test.ts`, `web/src/accounts.test.ts`, `web/src/home.test.ts`, `.claude/belege/issue29/` (Screenshots mit erfundenen Daten), diese Change-Artefakte.

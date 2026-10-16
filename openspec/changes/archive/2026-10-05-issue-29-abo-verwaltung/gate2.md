# Gate 2

## Roter Vorlauf
Branch `issue-29/abo-frontend`, gemeinsamer Workspace. Tests vor jeder Implementierung geschrieben und ausgeführt.

`go test ./internal/upstream ./internal/accounts` rot (Kompilierfehler):
```
internal/upstream/profile_test.go:24:12: undefined: readProfile
internal/accounts/license_test.go:33:130: unknown field License in struct literal of type UsageSnapshot
internal/accounts/license_test.go:40:16: undefined: TokenValid
internal/accounts/license_test.go:67:60: undefined: TokenRelogin
internal/accounts/license_test.go:68:56: undefined: TokenExpired
FAIL github.com/niclasedge/tollgate/internal/upstream [build failed]
FAIL github.com/niclasedge/tollgate/internal/accounts [build failed]
```

`pnpm test` rot: `Test Files 4 failed | 9 passed (13)`, `Tests 6 failed | 111 passed (117)`:
```
FAIL src/license.test.ts: Failed to resolve import "./lib/license"
FAIL account-usage.test.ts > shows the five-hour window as a subtle bar … : kein meter "5-Stunden-Verbrauch"
FAIL accounts.test.ts > shows token state, plan and a dated license … : kein article "Erstes Abo"
FAIL accounts.test.ts > offers a new login for an inactive subscription … : kein article "Zweites Abo"
FAIL accounts.test.ts > cancels the inline confirmation with Escape … : Bestätigung bleibt offen
FAIL accounts.test.ts > acceptance: switches from subscription 1 to 2 … : kein article "Zweites Abo"
FAIL home.test.ts > Übersicht mit Account-Verwaltung > shows the same card per subscription … : kein article
```

## Grüner Lauf nach Umsetzung
Geprüft im gemeinsamen Workspace (Kombination aller aktiven Branches) für `issue-29/abo-frontend`:
- `make lint` grün (gofmt, `go vet ./...`).
- `make test` grün: Svelte-Check 461 Dateien, 0 Fehler, 0 Warnungen; `Test Files 13 passed (13)`, `Tests 120 passed (120)`; `go test -race ./...` 17 Pakete `ok`, darunter `internal/accounts` und `internal/upstream` mit den neuen Lizenz- und Profiltests.
- `make build` grün, `openspec validate issue-29-abo-verwaltung --strict` grün.
- TS-Typen per `make generate` (tygo) erzeugt, nicht von Hand.
- Screenshots mit erfundenen Daten (Mock-API, keine echten Accounts) unter `.claude/belege/issue29/`: Accounts desktop hell/dunkel, iPhone dunkel, iPhone hell mit Inline-Bestätigung, Übersicht desktop hell. iPhone (390 px) ohne horizontalen Überlauf.

Nicht geprüft: Profilfelder live gegen ein echtes Abo (bewusst kein Zugriff auf echte Accounts), kein unabhängiges Review, keine eigenständige Branch-/CI-Prüfung, kein Commit, Push oder PR.

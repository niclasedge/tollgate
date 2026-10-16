# Tasks

## 1. Umsetzung

- [x] 1.1 `server.Options.Admin`/`Frontend`, Routing in `ServeHTTP` ohne ServeMux (`TestRoutesAdminFrontendAndProxy`, `TestWithoutFrontendRootGoesToProxy`).
- [x] 1.2 `config.Admin.TokenFile` mit Default neben `db.path` und Env-Override (`TestLoadDefaultsAndValues`, `TestEnvOverridesFile`).
- [x] 1.3 `cmd/tollgate`: `auth.NewVerifier`, `auth.LoadOrCreateAdminToken`, `auth.NewAdminAPI`, `web.Handler()`; `keys.go` entfernt (`TestWiringFrontendAdminAPIAndProxy`).
- [x] 1.4 `tygo.yaml` für `internal/auth/types.go`, Ausgabe `web/src/lib/types/auth.ts`.
- [x] 1.5 Nebenbei: `TestDeclaredOversizedBodyIsRejectedBeforeReading` mit Kontext-Timeout, damit eine Regression scheitert statt zu hängen.

## 2. Prüfung

- [x] 2.1 `make lint`, `make test`, `make build` in frischer Kopie grün.
- [x] 2.2 `openspec validate issue-54-verdrahtung --strict` grün.
- [x] 2.3 PR #55 mit `Closes #54`, CI grün, gemergt.

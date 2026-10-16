# Tasks

## 1. Backend

- [x] 1.1 `meter.ParseLimits`, `meter.LimitTracker`. Tests: `TestParseLimits`, `TestLimitTracker`, `FuzzParseLimits`.
- [x] 1.2 `server.Options.ResponseObserver`, `dashboard` `Status.Limits`, Verdrahtung in `main.go`, tygo. Tests: `TestResponseObserver`, `TestStatusLimits`.

## 2. Frontend

- [x] 2.1 Abschnitt „Abo-Limits“ in Accounts. Tests (vitest): mit und ohne Daten, Reset-Format.

## 3. Abschluss

- [x] 3.1 `make lint`, `make test`, `make build`, `just smoke` grün; `openspec validate --strict`.
- [x] 3.2 PR mit `Closes #71`, CI grün, gemergt.

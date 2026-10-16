# Tasks

## 1. Umsetzung

- [x] 1.1 `Intake.Authenticate`, `Process` nutzt es.
- [x] 1.2 `Authenticator`-Interface und `Runner.Authenticate` (`TestRunnerAuthenticateUsesHeadersOnly`).
- [x] 1.3 Server prüft vor dem Body-Read (`TestUnauthenticatedRequestIsRejectedWithoutReadingBody`, Gegenprobe ohne Fix scheitert mit Timeout).

## 2. Prüfung

- [x] 2.1 `make lint`, `make test`, `make build` grün.
- [x] 2.2 `openspec validate issue-52-auth-vor-body --strict` grün.
- [x] 2.3 PR #64 mit `Closes #52`, CI grün, gemergt.

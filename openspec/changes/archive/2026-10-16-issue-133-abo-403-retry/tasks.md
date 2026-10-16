# Tasks

## 1. Umsetzung

- [x] 1.1 Tests zuerst (`account_denied_test.go`): 403 `oauth_not_allowed_for_organization` wird 503 mit `Retry-After`; anderer 403 bleibt unverändert; Trace trägt `account_not_allowed`.
- [x] 1.2 `accountDenied` erkennt den Fehler am Upstream-Body; `forward` antwortet mit 503 und loggt ohne Body.

## 2. Abschluss

- [x] 2.1 `make test`, `make lint` grün; deployt.

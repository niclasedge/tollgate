## Umsetzung
- [x] 1. Test `TestComposePassesRedactSwitch` in `deploy/compose_env_test.go` schreiben und rot sehen (Gate 2).
- [x] 2. `TOLLGATE_REDACT_ENABLED: ${TOLLGATE_REDACT_ENABLED:-false}` im Dienst `tollgate` ergänzen, `docs/DEPLOY.md` beschreibt das Umschalten.
- [x] 3. `make lint`, `make test`, `make build` und `openspec validate --strict` grün.

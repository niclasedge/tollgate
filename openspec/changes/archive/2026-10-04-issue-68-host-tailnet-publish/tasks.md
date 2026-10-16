# Tasks

## 1. tollgate

- [x] 1.1 `published` in `config` (YAML, `TOLLGATE_PUBLISHED`), `CheckBinding`, `server.Listen` und Log. Tests: `TestCheckBinding`, `TestValidate` (published-Fälle), `TestLoadEnv`.

## 2. Deployment

- [x] 2.1 Compose ohne Sidecar, Entrypoint mit `published`, Smoke-Override.
- [x] 2.2 `setup.sh`: Tailnet-IP des Hosts nach `.env`, kein Auth-Key; `just up` über `setup.sh`.
- [x] 2.3 ADR-0013, ADR-Index, ARCHITECTURE, DEPLOY, AGENTS-Hinweis zur Bindungsregel.

## 3. Abschluss

- [x] 3.1 `make lint`, `make test`, `make build`, `just smoke` grün; `openspec validate --strict`.
- [x] 3.2 PR mit `Closes #68`, CI grün, gemergt.

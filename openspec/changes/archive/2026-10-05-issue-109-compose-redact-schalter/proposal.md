## Why
`internal/config` liest `TOLLGATE_REDACT_ENABLED`, aber `docker-compose.yml` reicht die Variable nicht an den tollgate-Container durch. Im Container lässt sich die Redaktion daher nicht einschalten; Lauf B der Phase-2-Abnahme (#17) ist so nicht möglich. Issue: https://github.com/niclasedge/tollgate/issues/109.
## What Changes
Der Dienst `tollgate` in `docker-compose.yml` bekommt `TOLLGATE_REDACT_ENABLED: ${TOLLGATE_REDACT_ENABLED:-false}`. Der Default bleibt aus (#90 ändert ihn). `docs/DEPLOY.md` beschreibt das Umschalten über `.env` und `just up`.
## Capabilities
### Modified Capabilities
- `deployment`: Redact-Schalter im Container.
## Impact
Branch `issue-109/compose-redact-schalter`, gemeinsamer Workspace. Dateien: `docker-compose.yml`, `docs/DEPLOY.md`, `deploy/compose_env_test.go`, diese Artefakte. Kein Go-Code außer dem Test.

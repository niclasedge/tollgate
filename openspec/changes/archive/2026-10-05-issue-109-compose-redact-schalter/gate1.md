# Gate 1
Branch `issue-109/compose-redact-schalter`, gemeinsamer Workspace. Spec-Delta `deployment`: der Container erhält `TOLLGATE_REDACT_ENABLED` aus `.env`, Default `false`. Geplanter Test: `TestComposePassesRedactSwitch` liest `docker-compose.yml` und erwartet genau `${TOLLGATE_REDACT_ENABLED:-false}` im Dienst `tollgate`.

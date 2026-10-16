# Tasks

## 1. Umsetzung

- [x] 1.1 `deploy/tollgate/Dockerfile` und `entrypoint.sh` (Tailnet-IP, Secrets 0600, `setpriv`).
- [x] 1.2 `docker-compose.yml` mit Sidecar, CLIProxyAPI, tollgate, `cpa-login`; `.dockerignore`.
- [x] 1.3 `deploy/setup.sh` und versioniertes `justfile`.
- [x] 1.4 `deploy/compose.smoke.yml` und `deploy/smoke.sh`.
- [x] 1.5 ADR-0012, `docs/DEPLOY.md`, ARCHITECTURE und AGENT_SETUP angepasst.

## 2. Prüfung

- [x] 2.1 `shellcheck` grün, `docker compose config` für beide Varianten gültig.
- [x] 2.2 `just smoke` grün (1:20 min; Kernel-Modus von `tailscale0` unter OrbStack separat geprüft).
- [x] 2.3 `make lint`, `make test` und `openspec validate --all --strict` grün.
- [ ] 2.4 Echter Lauf mit Tailnet-Key und Login (Betreiber). Offen beim Archivieren; die Abnahme im echten Tailnet bestätigt #11.
- [x] 2.5 PR #58 mit `Closes #57`, CI grün, Merge, Archivierung.

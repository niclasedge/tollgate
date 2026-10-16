# Aufgaben — issue-75-cpa-egress

## Vor der Umsetzung (rot)

- [x] Test `deploy/cliproxyapi/entrypoint_test.go` führt den Entrypoint mit Attrappen aus und prüft Compose und Dockerfile
- [x] Gate 2 bestätigt den roten Lauf

## Umsetzung (grün)

- [x] `deploy/cliproxyapi/entrypoint.sh` mit Egress-Regeln, Fail-closed, `setpriv` und `-local-model`
- [x] Dockerfile (`iptables`, Entrypoint, kein `USER`) und `docker-compose.yml` (`NET_ADMIN`, Login über Entrypoint)
- [x] `deploy/smoke.sh` prüft Egress und Abbruch ohne `NET_ADMIN`
- [x] ADR-0017, ADR-Index, `docs/DEPLOY.md` und `deploy/cliproxyapi/UPSTREAM.md` nachgezogen
- [x] `make lint`, `make test`, `make build`, `docker build` des CPA-Images und `just smoke` grün

# Aufgaben — issue-72-cpa-upstream-pin

## Vor der Umsetzung (rot)

- [x] Test `internal/upstream/pin_test.go` hält Pin, fehlende Kopie, Compose ohne `cpa-src` und Herkunftsdoku fest; `cmd/tollgate/main_test.go` prüft auf das CLIProxyAPI-Modul statt auf `third_party/`
- [x] `just gate2 issue-72-cpa-upstream-pin main issue-72/cpa-upstream-pin` bestätigt den roten Lauf

## Umsetzung (grün)

- [x] Dockerfile baut per `go install …@${CPA_VERSION}`, `cpa-src` aus `docker-compose.yml` entfernt
- [x] `make cpa` liest den Pin aus dem Dockerfile; CI baut am Pin und lässt den Smoke-Test laufen
- [x] `deploy/cliproxyapi/UPSTREAM.md` angelegt, `third_party/NOTICE.md` angepasst, `third_party/cliproxyapi/` gelöscht
- [x] ADR-0014, Status-Hinweise in ADR-0001/0002, ADR-Index, AGENTS.md und ARCHITECTURE.md nachgezogen
- [x] `make lint`, `make test`, `make cpa` mit Smoke-Test und `docker compose build cliproxyapi` grün

Der Abschluss steht bewusst nicht als Aufgabe: `just fertig issue-72-cpa-upstream-pin` verlangt, dass
keine Aufgabe mehr offen ist. Den Abschluss belegen gate1.md, gate2.md und fertig.md.

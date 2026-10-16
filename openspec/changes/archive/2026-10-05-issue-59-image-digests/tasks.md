# Aufgaben — issue-59-image-digests

## Vor der Umsetzung (rot)

- [x] Test `deploy/baseimages_test.go` prüft alle Dockerfiles auf `image:tag@sha256:<64 hex>` und erlaubt Stage-Verweise
- [x] Gate 2 bestätigt den roten Lauf

## Umsetzung (grün)

- [x] `deploy/tollgate/Dockerfile` und `deploy/cliproxyapi/Dockerfile` mit Tag und Index-Digest
- [x] Update-Ablauf der Basis-Images in `docs/DEPLOY.md`
- [x] `make lint`, `make test`, `make build`, `openspec validate --strict` und `docker build` beider Images grün

Stand der letzten Aufgabe: alles grün außer `docker build` des tollgate-Images. Das scheitert
schon auf `main` im Schritt `go build` (Root-Paket `fixtures.go` und `config/schema` werden
nicht kopiert, `testdata` steht in `.dockerignore`) und ist unabhängig von den Digests; die
Stages mit `node` und `golang` laufen mit den gepinnten Digests durch, das CPA-Image baut
vollständig. Eigenes Issue nötig.

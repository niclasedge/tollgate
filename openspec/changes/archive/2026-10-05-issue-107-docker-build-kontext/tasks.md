# Aufgaben — issue-107-docker-build-kontext

## Vor der Umsetzung (rot)

- [x] Test `deploy/buildcontext_test.go` prüft die Build-Eingaben von `./cmd/tollgate` gegen `COPY`-Zeilen und `.dockerignore`
- [x] Gate 2 bestätigt den roten Lauf

## Umsetzung (grün)

- [x] `deploy/tollgate/Dockerfile` kopiert `fixtures.go`, `config/` und `testdata/fixtures/`; `.dockerignore` gibt nur `testdata/fixtures` frei
- [x] `docker build -f deploy/tollgate/Dockerfile .` läuft durch (eigener Tag, danach entfernt)
- [x] `make lint`, `make test`, `make build` und `openspec validate --strict` grün

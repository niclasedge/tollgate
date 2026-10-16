# Tasks

## 1. Tests zuerst

- [x] 1.1 `internal/upstream/client_test.go` (tabellengetrieben, httptest) geschrieben und rot bestätigt (`undefined: New`).
- [x] 1.2 `internal/upstream/smoke_test.go` gegen das echte Binary (übersprungen ohne `TOLLGATE_CPA_BIN`).

## 2. Umsetzung

- [x] 2.1 `internal/upstream/client.go`; `go test -race`, `go vet`, `gofmt -l` grün.
- [x] 2.2 Kopie per `git archive` des Commits `8ef43e4…`, Abweichungen angewendet, `PATCHES.md` und `NOTICE.md` geschrieben; `CGO_ENABLED=0 go build ./cmd/server` grün, `go list ./...` im Root ohne Kopie.
- [x] 2.3 `deploy/cliproxyapi/{config.yaml,render-config.sh,Dockerfile}` und `docker-compose.yml`; `docker compose config` grün.
- [x] 2.4 Smoke-Test gegen das gebaute Binary grün (Health, Management 401/401/200, keine Nicht-Loopback-Erreichbarkeit).

## 3. Abschluss

- [x] 3.1 `openspec validate issue-3-cliproxyapi --strict` grün.
- [x] 3.2 `docker compose build` und `up`: Container wird `healthy` (HEALTHCHECK auf `/healthz`), läuft als `cpa` (uid 10001), veröffentlicht keinen Port, `config.yaml` mit 0600.
- [x] 3.3 `/secrets/` in `.gitignore`. Smoke-Test und CPA-Build in Makefile und CI: bewusst nicht in diesem Change (Smoke-Test braucht das 90-MB-Binary, läuft manuell über `TOLLGATE_CPA_BIN`).
- [x] 3.4 Review, PR mit `Closes #3`, CI grün, Merge, Archivierung.

## 4. Review-Nacharbeit

- [x] 4.1 Image läuft ohne root (`USER cpa`) und hat einen HEALTHCHECK auf `/healthz` (bash `/dev/tcp`, kein curl im Image).
- [x] 4.2 `render-config.sh` bricht ab, wenn das auth-dir mehr als eine Token-Datei (`*.json`) enthält (harte Regel „nur der aktive Account“).
- [x] 4.3 auth-dir-Pfad per Whitelist `[A-Za-z0-9_./-]` geprüft.
- [x] 4.4 `third_party/cliproxyapi/.gitkeep` aus dem Gerüst entfernt, die Kopie entspricht damit der Abweichungsliste in `PATCHES.md`.

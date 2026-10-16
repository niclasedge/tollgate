## REMOVED Requirements

### Requirement: Herkunft und Lizenz der Kopie

**Reason**: Die Kopie unter `third_party/cliproxyapi` enthält keinen Patch und wird durch einen gepinnten Build aus dem Upstream ersetzt (ADR-0014).

**Migration**: CLIProxyAPI wird mit `make cpa` oder im Container-Build per `go install …@${CPA_VERSION}` gebaut; Herkunft und Update-Ablauf stehen in `deploy/cliproxyapi/UPSTREAM.md`.

## ADDED Requirements

### Requirement: Gepinnter Upstream ohne Kopie

Das Repository SHALL keinen Quellcode von CLIProxyAPI enthalten. `deploy/cliproxyapi/Dockerfile` SHALL CLIProxyAPI mit `CGO_ENABLED=0` per `go install github.com/router-for-me/CLIProxyAPI/v8/cmd/server@${CPA_VERSION}` bauen, und der Commit MUST genau einmal als `ARG CPA_VERSION=` mit 40 Hex-Zeichen dort stehen. `docker-compose.yml` MUST keinen Build-Kontext auf eine lokale CLIProxyAPI-Quelle setzen. Das `Makefile` MUST den Pin aus dem Dockerfile lesen statt einen eigenen Commit zu nennen. `deploy/cliproxyapi/UPSTREAM.md` und `third_party/NOTICE.md` MUST denselben Commit, die MIT-Lizenz und beide Copyright-Zeilen nennen; `UPSTREAM.md` MUST den Update-Ablauf und das Anlegen der lokalen Referenz `ref/cliproxyapi` beschreiben. Das Root-Modul MUST kein Paket von CLIProxyAPI importieren.

#### Scenario: Keine Kopie im Repo
- **WHEN** das Repository geprüft wird
- **THEN** versioniert es keine Datei unter `third_party/cliproxyapi`, dort liegen weder `go.mod`, `LICENSE` noch `cmd/`, und `docker-compose.yml` enthält weder `cpa-src` noch `third_party/cliproxyapi`.

#### Scenario: Pin an genau einer Stelle
- **WHEN** ein Reviewer Dockerfile und Makefile liest
- **THEN** steht `ARG CPA_VERSION=` mit einem 40-stelligen Commit genau einmal im Dockerfile, der Build nutzt `go install …/cmd/server@${CPA_VERSION}` mit `CGO_ENABLED=0` ohne `COPY --from=cpa-src`, und das Makefile enthält keinen eigenen Commit-Hash.

#### Scenario: Herkunft ist nachvollziehbar
- **WHEN** ein Reviewer `UPSTREAM.md` und `NOTICE.md` liest
- **THEN** finden sich der Commit aus `CPA_VERSION`, die MIT-Lizenz, beide Copyright-Zeilen, der Update-Ablauf und `ref/cliproxyapi`.

#### Scenario: Binary aus dem Pin
- **WHEN** `make cpa` läuft und danach der Smoke-Test mit `TOLLGATE_CPA_BIN=bin/cli-proxy-api`
- **THEN** entsteht ein Binary ohne cgo, und der Smoke-Test besteht.

#### Scenario: Root-Modul getrennt
- **WHEN** `go list -deps ./cmd/tollgate` läuft
- **THEN** enthält die Liste kein Paket unter `github.com/router-for-me/CLIProxyAPI`.

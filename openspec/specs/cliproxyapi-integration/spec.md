# cliproxyapi-integration Specification

## Purpose
CLIProxyAPI ist als nachvollziehbar herkunftsgesicherte, nur lokal erreichbare Instanz eingebunden, und tollgate kann ihren Zustand prüfen.

## Requirements

### Requirement: Pflichtkonfiguration ohne Geheimnisse

`deploy/cliproxyapi/config.yaml` SHALL die Pflichtwerte aus ADR-0001 setzen: `server.host: 127.0.0.1`, genau einen `access.api-keys`-Eintrag, `requests.passthrough-headers: true`, `upstream.claude.disable-claude-cloak-mode: true`, `observability.logs.request-log: false`, ein Management-Secret, `management.allow-remote: false` und `management.disable-control-panel: true`. Die Vorlage MUST Platzhalter statt Geheimnisse enthalten. `render-config.sh` MUST Geheimnisse aus Dateien lesen und leere, zu kurze oder Sonderzeichen enthaltende Werte ablehnen.

#### Scenario: Gerenderte Konfiguration startet CLIProxyAPI
- **WHEN** die Vorlage mit zwei gültigen Secret-Dateien gerendert und CLIProxyAPI damit gestartet wird
- **THEN** antwortet `GET /healthz` mit 200 und CLIProxyAPI ist nur über Loopback erreichbar.

#### Scenario: Schwaches Geheimnis
- **WHEN** eine Secret-Datei leer ist, kürzer als 32 Zeichen oder Zeichen außerhalb von `[A-Za-z0-9_-]` enthält
- **THEN** bricht `render-config.sh` mit einem Fehlerstatus ab und schreibt keine Konfiguration mit diesem Wert.

### Requirement: Deployment ohne veröffentlichten Port

`docker-compose.yml` SHALL den Dienst `cliproxyapi` definieren, der keinen Port veröffentlicht und seine Geheimnisse ausschließlich über Compose-Secrets aus Dateien erhält.

#### Scenario: Compose-Validierung
- **WHEN** `docker compose config` mit vorhandenen Secret-Dateien ausgeführt wird
- **THEN** ist die Konfiguration gültig und enthält keinen `ports`-Eintrag und keine Geheimnisse im Klartext.

### Requirement: Upstream-Client mit Health-Check

`internal/upstream` SHALL einen Client bereitstellen, der nur `http://`-Basis-URLs mit Loopback-Host ohne Pfad und Userinfo akzeptiert. `Health` MUST `GET /healthz` ohne Zugangsdaten senden und nur bei Status 200 und `{"status":"ok"}` erfolgreich sein. Der Client MUST keinen Weiterleitungen folgen, und Fehler MUST keinen Antwortkörper enthalten.

#### Scenario: Nicht-Loopback-Ziel
- **WHEN** `New` mit einer LAN-, Tailnet- oder öffentlichen Adresse aufgerufen wird
- **THEN** wird ein Fehler zurückgegeben und kein Client erzeugt.

#### Scenario: Gesunder Upstream
- **WHEN** der Upstream auf `/healthz` mit 200 und `{"status":"ok"}` antwortet
- **THEN** gibt `Health` `nil` zurück.

#### Scenario: Kranker oder unerreichbarer Upstream
- **WHEN** der Upstream 500, ungültiges JSON, einen anderen Status-Wert, eine Weiterleitung liefert, nicht erreichbar ist oder der Kontext abläuft
- **THEN** gibt `Health` einen Fehler ohne Antwortkörper zurück.

#### Scenario: Echter Upstream
- **WHEN** das gebaute CLIProxyAPI-Binary mit der gerenderten Vorlage läuft und `TOLLGATE_CPA_BIN` gesetzt ist
- **THEN** besteht der Health-Check, `/v8/management/credentials` antwortet ohne Schlüssel und mit dem tollgate-Schlüssel mit 401 und mit dem Management-Schlüssel mit 200.

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

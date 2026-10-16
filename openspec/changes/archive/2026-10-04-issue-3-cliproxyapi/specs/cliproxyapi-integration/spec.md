# Spec Delta

## Purpose

CLIProxyAPI ist als nachvollziehbar herkunftsgesicherte, nur lokal erreichbare Instanz eingebunden, und tollgate kann ihren Zustand prüfen.

## ADDED Requirements

### Requirement: Herkunft und Lizenz der Kopie

Das Repository SHALL CLIProxyAPI als eigenständiges Go-Modul unter `third_party/cliproxyapi` enthalten, mit der unveränderten `LICENSE`. `third_party/cliproxyapi/PATCHES.md` MUST den Upstream-Commit nennen und jede Abweichung begründen; `third_party/NOTICE.md` MUST Quelle, Lizenz und Copyright-Zeilen nennen. Das Root-Modul MUST die Kopie weder importieren noch in `go list ./...` einschließen.

#### Scenario: Kopie ist baubar und getrennt
- **WHEN** in `third_party/cliproxyapi` `CGO_ENABLED=0 go build ./cmd/server` ausgeführt wird
- **THEN** entsteht ein Binary, und `go list ./...` im Repo-Root listet kein Paket unter `third_party/cliproxyapi`.

#### Scenario: Herkunft ist nachvollziehbar
- **WHEN** ein Reviewer `PATCHES.md` und `NOTICE.md` liest
- **THEN** finden sich Commit `8ef43e4df3b216a42493105d31c2873b69191473`, die MIT-Lizenz, beide Copyright-Zeilen und eine Begründung für jede Abweichung von der Upstream-Dateiliste.

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

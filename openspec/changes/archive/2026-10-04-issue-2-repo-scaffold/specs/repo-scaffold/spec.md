# Spec Delta

## Purpose

Das Repo-Gerüst ermöglicht Entwicklern, tollgate aus einem frischen Checkout zu bauen, zu prüfen und die spätere Typgenerierung mit denselben Befehlen wie in CI auszuführen.

## ADDED Requirements

### Requirement: Eigenständig baubares Gerüst

Das Repository SHALL mit `make build` ein ausführbares tollgate-Binary ohne cgo erzeugen. Der Build MUST ohne Frontend-Build, lokale Referenz-Repositories, CLIProxyAPI oder Zugangsdaten funktionieren.

#### Scenario: Build aus frischem Checkout
- **WHEN** ein Entwickler mit dokumentierter Go-Toolchain und Make `make build` ausführt
- **THEN** entsteht `bin/tollgate` ohne zusätzliche Laufzeitdienste oder Frontend-Artefakte.

### Requirement: Sicheres Startverhalten des Gerüsts

Das Gerüst-Binary SHALL beim Start einen strukturierten JSON-Logeintrag schreiben, seinen Gerüststatus kenntlich machen und erfolgreich enden. Es MUST ohne Zugangsdaten starten und darf keine Netzwerkverbindung oder Listener öffnen.

#### Scenario: Start ohne Konfiguration
- **WHEN** `bin/tollgate` ohne Konfiguration gestartet wird
- **THEN** erscheint ein JSON-Logeintrag mit Meldung und Log-Level, und der Prozess endet mit Exit-Code 0 ohne Netzwerkzugriff.

### Requirement: Gemeinsamer Testbefehl

`make test` SHALL alle vorhandenen Tests des tollgate-Go-Moduls mit `go test -race ./...` ausführen und bei einem Testfehler einen Fehlerstatus zurückgeben. Es MUST ohne Frontend-Werkzeuge funktionieren, solange noch keine Frontend-Tests existieren.

#### Scenario: Prüfung des Backend-Gerüsts
- **WHEN** `make test` vor Einführung des Frontends ausgeführt wird
- **THEN** werden alle tollgate-Go-Pakete mit aktiviertem Race Detector geprüft, ohne Node oder pnpm vorauszusetzen.

#### Scenario: Fehlgeschlagener Test
- **WHEN** ein Go-Test fehlschlägt
- **THEN** endet `make test` mit einem von null verschiedenen Exit-Code.

### Requirement: Reproduzierbare Typgenerierung

Das Repository SHALL `make generate` mit einer im Go-Modul gepinnten Werkzeugversion bereitstellen. Der Befehl MUST ohne globale Generatorinstallation funktionieren und wiederholt denselben versionierten Ausgabezustand erzeugen. Solange keine API-Datentypen existieren, MUST er erfolgreich ohne erfundene API-Typen abschließen.

#### Scenario: Generierung ohne API-Datentypen
- **WHEN** `make generate` im Gerüst zweimal hintereinander ausgeführt wird
- **THEN** enden beide Läufe erfolgreich und erzeugen keine fachlichen Typen oder unterschiedlichen versionierten Ausgaben.

### Requirement: Automatische Prüfung von Änderungen

GitHub Actions SHALL bei Pull Requests und Pushes auf `main` die Test-, Build- und Generierungsbefehle ausführen. Fehler in einem dieser Schritte MUST den Workflow fehlschlagen lassen; Abweichungen zwischen generierten und versionierten Ausgaben MUST erkannt werden.

#### Scenario: CI prüft das Gerüst
- **WHEN** ein Pull Request das Gerüst ändert
- **THEN** prüft CI `make test`, `make build` und `make generate` ohne Secrets, lokale Referenzen oder gestartete Upstream-Dienste.

#### Scenario: Veraltete generierte Ausgabe
- **WHEN** die Generierung versionierte Ausgaben verändert oder neue nicht ignorierte Ausgaben erzeugt
- **THEN** meldet CI einen Fehler.

### Requirement: Dokumentierte Entwicklungsumgebung

Das Repository SHALL die benötigten Werkzeuge, Befehle, Ausgabeorte und den aktuellen Gerüstumfang dokumentieren. Generierte Build-Ausgaben MUST von der Versionierung ausgeschlossen bleiben; der versionierte Platzhalter unter `web/dist` MUST im frischen Checkout vorhanden sein.

#### Scenario: Entwickler richtet das Repository ein
- **WHEN** ein Entwickler die Entwicklungsdokumentation liest
- **THEN** kann er Build, Tests und Generierung ausführen und erkennt, dass Proxy und Frontend erst in Folge-Issues implementiert werden.

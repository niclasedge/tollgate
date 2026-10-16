## ADDED Requirements

### Requirement: Statische Prüfung

Das Repository SHALL mit `make lint` alle Go-Pakete auf Formatierung (`gofmt`) und `go vet`-Befunde prüfen. Der Befehl MUST mit einem von null verschiedenen Exit-Code enden, wenn eine Datei nicht formatiert ist oder `go vet` einen Befund meldet, und die CI MUST ihn ausführen.

#### Scenario: Nicht formatierte Datei
- **WHEN** eine Go-Datei nicht `gofmt`-formatiert ist
- **THEN** nennt `make lint` die Datei und endet mit einem Fehlerstatus.

#### Scenario: Sauberer Stand
- **WHEN** alle Dateien formatiert sind und `go vet` nichts meldet
- **THEN** endet `make lint` mit Exit-Code 0.

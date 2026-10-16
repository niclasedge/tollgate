# Spec Delta

## ADDED Requirements

### Requirement: Rechte vorhandener Sidecar-Dateien

Der Store SHALL beim Öffnen vorhandene `-wal`- und `-shm`-Dateien, die gruppen- oder weltweit zugänglich sind, auf Modus 0600 setzen.

#### Scenario: Alte Installation mit 0644-Sidecars
- **WHEN** `-wal` und `-shm` mit Modus 0644 neben der Datenbank liegen und der Store geöffnet wird
- **THEN** haben beide danach den Modus 0600.

### Requirement: Trace-Liste ohne Temp-Sort

Die Abfrage von `ListTraces` SHALL für alle Filterkombinationen (mit und ohne `ClientID`, mit und ohne Cursor) über einen Index sortiert werden. Der Query-Plan MUST keinen `TEMP B-TREE` enthalten.

#### Scenario: Keyset-Abfrage mit Client-Filter
- **WHEN** `EXPLAIN QUERY PLAN` für `ClientID`, `Before` und `BeforeID` ausgeführt wird
- **THEN** nutzt der Plan `traces_client` und keinen `TEMP B-TREE`.

#### Scenario: Keyset-Abfrage ohne Client-Filter
- **WHEN** `EXPLAIN QUERY PLAN` für `Before` und `BeforeID` ohne `ClientID` ausgeführt wird
- **THEN** nutzt der Plan `traces_started_at` und keinen `TEMP B-TREE`.

### Requirement: Paralleles Öffnen einer frischen Datei

`Open` SHALL auch dann erfolgreich sein, wenn mehrere Stores gleichzeitig dieselbe neue Datei öffnen, und ein kurzzeitiges `SQLITE_BUSY` beim ersten Verbindungsaufbau mit Wiederholung überbrücken.

#### Scenario: Vier gleichzeitige Opener
- **WHEN** vier Stores gleichzeitig dieselbe neue Datei öffnen
- **THEN** sind alle `Open` erfolgreich und `schema_migrations` enthält jede Version genau einmal.

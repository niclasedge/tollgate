# cpa-refresh-barrier Specification

## Purpose
Eine begrenzte Refresh-Barriere im Image-Build von CLIProxyAPI verhindert, dass ein Token-Refresh während eines Account-Tauschs die alte Token-Datei zurückschreibt.

## Requirements

### Requirement: Refresh-Pause
Die authentifizierte CPA-Management-API SHALL vor Bestätigung einer Pause laufende Refreshes und deren Speicherung abwarten und weitere Refreshes verhindern.
#### Scenario: Gleichzeitige Rotation
- **WHEN** ein Refresh beim Wechsel noch läuft
- **THEN** wartet Pause auf seinen Mutex und der folgende Snapshot enthält den neuesten Token
#### Scenario: Pausierter Account
- **WHEN** nach Pause ein weiterer Refresh gestartet wird
- **THEN** wird kein Refresh-Netzaufruf ausgeführt, bis Resume erfolgt

### Requirement: Gepinnter Build
Der Build SHALL die begrenzte Erweiterung ausschließlich auf temporäre Quellen des gepinnten CPA anwenden und die erwarteten Einfügestellen validieren.
#### Scenario: Upstream ändert sich
- **WHEN** eine Einfügestelle fehlt oder mehrfach vorkommt
- **THEN** bricht der Build ab

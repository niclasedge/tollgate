## ADDED Requirements
### Requirement: Unveränderliche Account-Zuordnung
Jede Anfrage SHALL nach Eintritt in die Wechsel-Sperre einmalig die tatsächliche aktive Account-ID lesen und diese unverändert in SQLite, Trace-API und SSE sichern. Client-Eingaben SHALL die Zuordnung nicht beeinflussen. Ein unbekannter Account SHALL leer bleiben, ein fehlgeschlagener Identitäts-Read SHALL vor Weiterleitung abgewiesen werden.
#### Scenario: Stream und Wechsel
- **WHEN** eine Anfrage läuft und ein Wechsel versucht wird
- **THEN** bleibt der Wechsel gesperrt und der Trace behält dieselbe ID vom Start bis zum Abschluss
#### Scenario: Nächster Account
- **WHEN** nach einem erfolgreichen Account-Wechsel eine neue Anfrage beginnt
- **THEN** erhält nur die neue Anfrage die neue Account-ID
#### Scenario: Bestehende oder gelöschte Accounts
- **WHEN** alte Traces migriert werden oder ein inaktiver Account gelöscht wird
- **THEN** bleiben alte Traces unzugeordnet und bereits gespeicherte Account-IDs unverändert erhalten
### Requirement: Auswertbare Tokens und Preisvergleich
Die API SHALL Verbrauch und Preisvergleich zusätzlich nach Account getrennt aggregieren. Das Frontend SHALL die Account-ID in Live/Details und eine Pro-Account-Auswertung anzeigen; unzugeordnete Anfragen SHALL separat bleiben.
#### Scenario: Zwei Accounts, gleicher Client und Modell
- **WHEN** beide Accounts am gleichen Tag mit demselben Client und Modell verwendet werden
- **THEN** entstehen getrennte Summen je Account, ohne Veränderung der Gesamtsumme

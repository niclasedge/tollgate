## ADDED Requirements

### Requirement: Abo-Limits in Accounts

Die Accounts-Ansicht SHALL für das 5-Stunden-Fenster und die Woche je die Auslastung in Prozent als Balken, den Status als runden Indikator und die Zeit bis zum Reset mit Uhrzeit zeigen. Ohne Messung MUST sie „Noch keine Daten“ zeigen, statt Werte zu raten.

#### Scenario: Limits vorhanden
- **WHEN** `/api/status` `five_hour.utilization: 0.42` mit einem Reset in 2 h 13 min liefert
- **THEN** zeigt Accounts „42 %“ und „zurückgesetzt in 2 h 13 min“.

#### Scenario: Keine Daten
- **WHEN** `limits` `null` ist
- **THEN** zeigt Accounts „Noch keine Daten“.

## ADDED Requirements

### Requirement: Startseite Übersicht

Die Übersicht SHALL auf einen Blick zeigen: die Abo-Limits als Karte (5 Stunden und Woche je mit Prozent, Balken und „zurückgesetzt in …“ mit Datum), die Kennzahlen des heutigen UTC-Tages aus `/api/costs?days=1` (Anfragen, Tokens ein und aus, Cache-Trefferquote, gesparte Tokens), die Health von tollgate und CLIProxyAPI mit dem aktiven Account sowie die letzten 10 Anfragen mit Link ins Detail. Fehlt ein Wert, MUST sie „unbekannt“ oder „Noch keine Daten“ zeigen statt zu raten. Die Limit-Karte MUST als eigene Komponente für genau ein Abo nutzbar sein.

#### Scenario: Limits vorhanden
- **WHEN** `/api/status` `five_hour.utilization: 0.42` mit einem Reset in 2 h 13 min liefert
- **THEN** zeigt die Karte „42 %“ und „zurückgesetzt in 2 h 13 min“ mit Datum.

#### Scenario: Keine Limits
- **WHEN** `limits` `null` ist
- **THEN** zeigt die Karte „Noch keine Daten“ und keinen Balken.

#### Scenario: Letzte Anfrage öffnen
- **WHEN** der Nutzer auf eine der letzten Anfragen klickt
- **THEN** öffnet sich Live mit dieser Anfrage aufgeklappt.

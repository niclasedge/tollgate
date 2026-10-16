## ADDED Requirements

### Requirement: 5-h-Auslastung am Trace

Der Store SHALL am Trace die 5-h-Auslastung in Promille (`five_hour_permille`, ganzzahlig, nicht negativ) und den Reset-Zeitpunkt des 5-h-Fensters (`five_hour_reset`, Unix-Sekunden) speichern. Beide Werte MUST NULL bleiben, wenn die Antwort sie nicht enthält. Der Store MUST die Zeilen für die Fensterauswertung ab einem Zeitpunkt nach `started_at` sortiert liefern.

#### Scenario: Werte aus der Antwort
- **WHEN** ein Trace mit Auslastung 0,423 und Reset 1760000000 abgeschlossen wird
- **THEN** stehen am Trace 423 und 1760000000.

#### Scenario: Antwort ohne Limit-Header
- **WHEN** ein Trace ohne diese Werte abgeschlossen wird
- **THEN** bleiben beide Spalten NULL.

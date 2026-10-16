## ADDED Requirements

### Requirement: 7-Tage-Auslastung am Trace

Der Store SHALL am Trace die 7-Tage-Auslastung in Promille (`seven_day_permille`, ganzzahlig, nicht negativ) und den Reset-Zeitpunkt des 7-Tage-Fensters (`seven_day_reset`, Unix-Sekunden) speichern und in den Zeilen der Fensterauswertung liefern. Fehlen die Werte in der Antwort, MUST beide NULL bleiben.

#### Scenario: Werte aus der Antwort
- **WHEN** ein Trace mit 7-Tage-Auslastung 0,31 und Reset 1760500000 abgeschlossen wird
- **THEN** stehen am Trace 310 und 1760500000, und die Fensterauswertung liefert beide.

### Requirement: Preis-Grundlage in der Fensterauswertung

Die Zeilen der Fensterauswertung SHALL Modell und Usage (Eingabe, Ausgabe, Cache-Lesen, Cache-Schreiben; fehlende Werte als 0) des Traces liefern.

#### Scenario: Usage in der Zeile
- **WHEN** ein Trace mit Usage 5/7/11/13 abgeschlossen wird
- **THEN** liefert die Fensterauswertung Modell und genau diese Usage; ein Trace ohne Usage liefert 0.

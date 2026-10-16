## ADDED Requirements

### Requirement: Tokens/s-Urteil in Traces

`TraceSummary` SHALL ein Feld `tps` mit Urteil, Klasse, Anzahl Vergleichswerte, Tokens/s mit einer Nachkommastelle, p10, Median, p90 und Abweichung in Promille tragen. Ohne gespeichertes Urteil MUST `tps` `null` sein. Das `trace`-Event nach dem Abschluss MUST das Urteil bereits enthalten.

#### Scenario: Urteil im Event
- **WHEN** eine Stream-Anfrage abgeschlossen und beurteilt wird
- **THEN** enthält das `trace`-Event `tps.verdict` und `tps.median`.

#### Scenario: Alter Trace
- **WHEN** ein Trace vor der Migration abgeschlossen wurde
- **THEN** ist `tps` `null`.

### Requirement: Einfluss und Trend im Performance-Bericht

`GET /api/performance` SHALL je Modell ein Feld `influence` mit allen 12 Zellen aus Eingabe- und Cache-Klasse (sortiert nach Eingabe-, dann Cache-Klasse) und den Kernaussagen liefern. Jeder Tag im Verlauf SHALL zusätzlich den Median Tokens/s und den Median Cache-Anteil in Promille tragen, `null` ohne Werte. Speedtest-Traces MUST NOT eingehen.

#### Scenario: Leere Zellen
- **WHEN** ein Modell nur Anfragen der Klasse `i0` hat
- **THEN** enthält `influence.cells` trotzdem 12 Zellen, die übrigen mit `samples: 0`.

#### Scenario: Trend je Tag
- **WHEN** an einem Tag drei Stream-Anfragen mit 40, 50 und 60 Tokens/s liegen
- **THEN** meldet dieser Tag einen Median von 50 Tokens/s.

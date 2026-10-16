## ADDED Requirements
### Requirement: Letzte 5 Minuten auf der Übersicht
Die Übersicht SHALL im Abschnitt „Letzte 5 Minuten“ die Zahl der Anfragen aus `/api/calls`, den Ø der 10 Fenster davor und einen runden Statusindikator mit „über Ø“, „unter Ø“ oder „wie Ø“ zeigen. Der Abschnitt MUST mit den übrigen Daten der Übersicht neu laden (Intervall und „Aktualisieren“). Schlägt der Abruf fehl, MUST der Fehler in diesem Abschnitt stehen, ohne die anderen Abschnitte zu verbergen.

#### Scenario: Über dem Durchschnitt
- **WHEN** `/api/calls` `current: 4`, `average: 2.3` und `trend: "above"` liefert
- **THEN** zeigt der Abschnitt „4“, „Ø 2,3 je 5 min in den 50 min davor“ und einen Indikator „über Ø“.

#### Scenario: Unter dem Durchschnitt
- **WHEN** `/api/calls` `trend: "below"` liefert
- **THEN** zeigt der Indikator „unter Ø“.

#### Scenario: Fehler
- **WHEN** `/api/calls` mit einem Fehler antwortet
- **THEN** steht die Meldung im Abschnitt „Letzte 5 Minuten“ und die übrigen Abschnitte bleiben sichtbar.

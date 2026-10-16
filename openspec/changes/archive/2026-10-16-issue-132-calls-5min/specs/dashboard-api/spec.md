## ADDED Requirements
### Requirement: Anfragen der letzten fünf Minuten
Die Dashboard-API SHALL unter `GET /api/calls` hinter dem Admin-Token aus dem Store liefern: die Zahl der Traces im aktuellen Fenster [jetzt−5 min, jetzt), die Summe der Traces in den 10 Fenstern davor [jetzt−55 min, jetzt−5 min), deren Durchschnitt je Fenster mit einer Nachkommastelle und die Einordnung `above`, `below` oder `equal` des aktuellen Fensters gegen diesen Durchschnitt. Leere Fenster MUST als 0 zählen. Jeder Trace zählt, gleich welcher Endpunkt oder Status. Die Antwort MUST NOT Inhalte, Modelle oder Clients enthalten.

#### Scenario: Mehr als üblich
- **WHEN** in den letzten 5 Minuten 4 Anfragen und in den 50 Minuten davor 20 Anfragen eingingen
- **THEN** liefert `/api/calls` `current: 4`, `previous_total: 20`, `average: 2` und `trend: "above"`.

#### Scenario: Fenstergrenzen
- **WHEN** Traces genau bei jetzt−5 min, genau bei jetzt−55 min, kurz vor jetzt−55 min und genau bei jetzt liegen
- **THEN** zählt der erste zum aktuellen Fenster, der zweite zu den Vorfenstern und die beiden anderen gar nicht.

#### Scenario: Keine Anfragen
- **WHEN** in der letzten Stunde keine Anfrage einging
- **THEN** liefert `/api/calls` `current: 0`, `previous_total: 0`, `average: 0` und `trend: "equal"`.

#### Scenario: Nur Admin
- **WHEN** `/api/calls` ohne Admin-Token oder mit einem Client-Key abgerufen wird
- **THEN** antwortet die API mit 401.

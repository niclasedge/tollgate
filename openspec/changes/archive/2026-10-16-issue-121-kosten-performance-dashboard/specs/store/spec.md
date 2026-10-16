## ADDED Requirements

### Requirement: Kosten je Stunde

Der Store SHALL abgeschlossene Traces ab einem Zeitpunkt je UTC-Stunde (`YYYY-MM-DDTHH:00:00Z`), Account und Modell mit Anzahl und Usage zusammenfassen, sortiert nach Stunde, Account und Modell. Traces ohne Account MUST unter leerem Account erscheinen, Traces ohne Usage MUST als Anfrage ohne Tokens zählen, und laufende Traces MUST fehlen.

#### Scenario: Stundengrenze
- **WHEN** ein Trace um 04:59:59.999 UTC und einer um 05:00:00.000 UTC startet
- **THEN** liegt der erste in `…T04:00:00Z` und der zweite in `…T05:00:00Z`.

#### Scenario: Laufender Trace
- **WHEN** ein Trace noch keinen Status hat
- **THEN** erscheint er in keiner Stunde.

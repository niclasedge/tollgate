## ADDED Requirements
### Requirement: Maske in Trace-Detail und letzten Redactions
`GET /api/traces/{id}` SHALL je Treffer und `GET /api/redactions` SHALL je Redaction das Feld `mask` liefern, bei älteren Traces `""`. Die Antworten MUST NOT Klartext-Werte enthalten.

#### Scenario: Maske ohne Klartext
- **WHEN** eine Anfrage mit einer erfundenen E-Mail-Adresse durch den Proxy lief
- **THEN** enthalten Trace-Detail und `/api/redactions` zum Platzhalter eine nicht leere Maske, aber weder die Adresse noch einen anderen Kanarienwert

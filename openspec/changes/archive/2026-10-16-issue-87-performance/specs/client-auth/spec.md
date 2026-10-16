## ADDED Requirements

### Requirement: Interner Speedtest-Client verborgen

Die Admin-API für Client-Keys SHALL den reservierten Client `speedtest` nicht in `GET /api/clients` liefern und MUST Sperren und Rotieren dieser ID mit 404 ablehnen. Anlegen mit einem Namen mit Präfix `tollgate:` MUST mit 400 abgelehnt werden.

#### Scenario: Liste ohne internen Client
- **WHEN** zwei Clients angelegt sind und die Migration den Speedtest-Client angelegt hat
- **THEN** liefert `GET /api/clients` genau zwei Einträge.

#### Scenario: Rotation verweigert
- **WHEN** der Admin `POST /api/clients/speedtest/rotate` aufruft
- **THEN** antwortet tollgate mit 404, und es entsteht kein Key.

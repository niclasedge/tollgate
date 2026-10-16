## ADDED Requirements
### Requirement: Redact-Schalter im Container
Der Dienst `tollgate` in `docker-compose.yml` SHALL die Variable `TOLLGATE_REDACT_ENABLED` aus `.env` an den Container durchreichen, mit dem Default `false`, solange #90 den Default nicht ändert.

#### Scenario: Redaktion per .env eingeschaltet
- **WHEN** in `.env` `TOLLGATE_REDACT_ENABLED=true` steht und `just up` läuft
- **THEN** startet tollgate mit eingeschalteter Redact-Stufe

#### Scenario: Ohne Eintrag
- **WHEN** `.env` die Variable nicht setzt
- **THEN** erhält der Container `TOLLGATE_REDACT_ENABLED=false` und die Redaktion bleibt aus

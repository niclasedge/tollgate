## ADDED Requirements

### Requirement: Schalter für die Redact-Stufe

Die Bootstrap-Konfiguration SHALL den Schalter `redact.enabled` (Umgebung `TOLLGATE_REDACT_ENABLED`) lesen. Der Standard MUST `false` sein, solange der Rückweg (#14) fehlt. Ein Umgebungswert, der kein Wahrheitswert ist, MUST mit einer Fehlermeldung abgelehnt werden, die den Variablennamen nennt. Ist der Schalter an, SHALL tollgate die Stufe `redact` nach dem Eingang einhängen und beim Start warnen.

#### Scenario: Standard
- **WHEN** weder Datei noch Umgebung den Schalter setzen
- **THEN** ist `redact.enabled` aus.

#### Scenario: Umgebung gewinnt
- **WHEN** die Datei `redact.enabled: false` und die Umgebung `TOLLGATE_REDACT_ENABLED=true` setzt
- **THEN** ist der Schalter an.

#### Scenario: Ungültiger Wert
- **WHEN** `TOLLGATE_REDACT_ENABLED=vielleicht` gesetzt ist
- **THEN** schlägt `Load` mit einer Meldung fehl, die `TOLLGATE_REDACT_ENABLED` nennt.

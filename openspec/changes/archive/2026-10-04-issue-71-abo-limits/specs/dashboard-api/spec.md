## ADDED Requirements

### Requirement: Abo-Limits im Status

`GET /api/status` SHALL das Feld `limits` liefern: `null` ohne Messung, sonst `status`, `reset`, `representative_claim`, `observed_at` sowie `five_hour` und `seven_day` mit je `utilization` (Anteil 0..1 oder `null`), `reset` und `status`. tollgate MUST die Werte nur aus den Headern weitergeleiteter Antworten nehmen und MUST NOT dafür eigene Aufrufe an Anthropic machen.

#### Scenario: Nach einer Anfrage
- **WHEN** eine weitergeleitete Antwort Unified-Header trug
- **THEN** enthält `/api/status` diese Werte und den Zeitpunkt der Messung in RFC 3339.

#### Scenario: Noch keine Anfrage
- **WHEN** seit dem Start keine Antwort mit Unified-Headern kam
- **THEN** ist `limits` `null`.

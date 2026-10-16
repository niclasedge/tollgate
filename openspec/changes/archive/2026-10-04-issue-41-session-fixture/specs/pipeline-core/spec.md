# Spec Delta

## ADDED Requirements

### Requirement: Sitzungsquelle an echter Anfrage belegt

Der Sitzungsschlüssel SHALL aus einer aufgezeichneten, bereinigten Claude-Code-Anfrage (`testdata/fixtures/`) abgeleitet werden können, sowohl aus dem Header als auch, ohne Header, aus dem JSON-Objekt in `metadata.user_id`.

#### Scenario: Fixture mit Header
- **WHEN** die Fixture unverändert durch Stufe 0 läuft
- **THEN** ist der Sitzungsschlüssel nicht leer und die Quelle `header`.

#### Scenario: Fixture ohne Header
- **WHEN** der Header `X-Claude-Code-Session-Id` entfernt wird
- **THEN** entsteht derselbe Sitzungsschlüssel mit Quelle `metadata`.

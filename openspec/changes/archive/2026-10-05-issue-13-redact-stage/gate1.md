# Gate 1

Branch `issue-13/redact-stage`, gemeinsamer Workspace. Spec-Deltas für `redaction`,
`pipeline-core`, `bootstrap-config` und `proxy` stehen; jedes Szenario hat einen geplanten
Test (Aufgaben 1.1 bis 1.5). `openspec validate issue-13-redact-stage --strict` ist grün.
Bewusste Abweichung vom Auftragswortlaut (Zuordnung pro Anfrage statt Sitzungs-TTL) folgt
ADR-0006 und ist in `design.md` begründet.

# Gate 1

Branch `issue-14/rueckweg`, gemeinsamer Workspace. Spec-Delta für `redaction` steht; jedes
Szenario hat einen geplanten Test (Aufgaben 1.1 bis 1.3). `openspec validate
issue-14-rueckweg --strict` ist grün. Abweichung zum ADR-Wortlaut „Zahl der Events bleibt
gleich“: Am Blockende kann ein synthetisches Delta hinzukommen, begründet in `design.md`.

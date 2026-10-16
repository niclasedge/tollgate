# Gate 1

Branch `issue-83/verdraengter-rest`, gemeinsamer Workspace. Spec-Delta für `redaction`
(Anforderung „Erst sammeln, dann auflösen“ um Restspannen ergänzt) steht; jedes neue Szenario
hat einen geplanten Test (Aufgaben 1.1 bis 1.3): Korpusfall
`tollgate/residual_of_displaced_generic`, vier Fälle in `TestResolution`, ein Fall in
`TestAllowlistWins`. Die Paritätsabweichung bleibt `overlap` und wird im ADR-0005-Nachtrag
begründet. `openspec validate issue-83-verdraengter-rest --strict` ist grün.

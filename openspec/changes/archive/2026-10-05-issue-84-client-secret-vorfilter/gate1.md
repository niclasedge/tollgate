# Gate 1

Branch `issue-84/client-secret-vorfilter`, gemeinsamer Workspace. Spec-Delta für
`redaction` mit zwei neuen Anforderungen (client_secret-Grenze, Vorfilter ohne
Trefferverlust) steht; jedes Szenario hat einen geplanten Test (Aufgaben 1.1 bis 1.3). Die
Leistung wird mit `BenchmarkFind` vorher und nachher gemessen (Aufgabe 1.4), die Zahlen
stehen in `design.md`. Kein Regex-Text ändert sich; die Abweichung zu Rust bleibt `prefix`.
`openspec validate issue-84-client-secret-vorfilter --strict` ist grün.

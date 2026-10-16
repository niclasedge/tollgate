# Gate 1

Branch `issue-15/redactor-konfiguration`, gemeinsamer Workspace. Spec-Deltas für
`redaction` (eigene Regex, Wortliste, Allowlist) und die neue Capability
`redaction-config` stehen; jedes Szenario hat einen geplanten Test (Aufgaben 1.1 bis 1.4).
`openspec validate issue-15-redactor-konfiguration --strict` ist grün. Bewusste
Abweichungen: Wortliste je Wort eine Regex statt einer Alternation (design.md), Export
standardmäßig ohne Wortliste (ADR-0008 erlaubt beide Varianten), Verschlüsselung über ein
Interface bis zum Merge von #28.

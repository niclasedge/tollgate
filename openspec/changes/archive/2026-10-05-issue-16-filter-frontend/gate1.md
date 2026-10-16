# Gate 1

Branch `issue-16/filter-frontend`, gemeinsamer Workspace. Spec-Deltas für `dashboard-api`,
`store`, `redaction-config` und `frontend-views` stehen; jedes Szenario hat einen geplanten
Test (Aufgaben 1.1 bis 1.6). `openspec validate issue-16-filter-frontend --strict` ist grün.

Bewusste Erweiterungen gegenüber dem Issue-Text, begründet in `design.md`: `words_from` in
`internal/redactcfg` (sonst kann das Formular die maskierte Wortliste nicht erhalten) und das
Feld `Placeholder` in `Match` (sonst kann die Übersicht keinen Platzhalter zeigen). Die
Rückweg-Zeile der Timeline nutzt vorhandene Daten; eine eigene Rückweg-Messung im Store ist
nicht Teil dieses Changes.

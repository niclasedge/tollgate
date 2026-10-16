# Gate 2

Branch `issue-17/phase2-abnahme`, gemeinsamer Workspace (geprüft wurde die Kombination aller
angewendeten Branches und uncommitteten Arbeiten, u. a. #91 parallel).

Rot vor der Umsetzung (ohne `testdata/fixtures/multi-turn-session.json`):

- `TestEndToEndFixtures`: `ran 2 fixtures, fixtureSecrets lists 3`.
- `TestNoPlaintextInDBLogsAndAdminAPI`: `ran 2 fixtures, fixtureSecrets lists 3`.
- `TestCacheStablePrefixAcrossTurns` war schon grün: Das Szenario prüft vorhandenes Verhalten
  (#13, #14, #15) über das ganze Binary; es gab keinen Fehler zu beheben.

Gegenprobe (nicht übernommen): Schreibt das Mock-Modell zusätzlich jeden Platzhalter ohne
Kontext, scheitert `TestCacheStablePrefixAcrossTurns` mit `upstream saw "fakepw-0000"` und
abweichender Assistant-Nachricht ab Turn 3. Befund in `design.md`, Issue #97.

Grün nach der Umsetzung (2026-10-05):

- `go test -race -count=3` der drei Abnahmetests grün.
- `make lint` grün.
- `make test`: Frontend 12 Dateien, 111 Tests grün, `svelte-check` ohne Fehler;
  `go test -race ./...` alle Pakete `ok`.
- `make build` grün.
- `openspec validate issue-17-phase2-abnahme --strict` grün.

# Tasks

## 1. Tests zuerst

- [x] 1.1 `cmd/tollgate/acceptance_test.go`: Mock-Modell (SSE, Deltas über Chunk-Grenzen), Testclient (baut Assistant-Nachricht wie Claude Code), `TestCacheStablePrefixAcrossTurns`.
- [x] 1.2 `TestEndToEndFixtures` mit Fixture-Tabelle (Geheimnisse, Allowlist) und Pflichteintrag pro Fixture.
- [x] 1.3 `TestNoPlaintextInDBLogsAndAdminAPI`: DB, WAL, SHM, Debug-Logs, Live-Stream, Admin-API.
- [x] 1.4 Rot sehen (Gate 2): ohne `multi-turn-session.json` scheitern E2E- und Leck-Test.

## 2. Umsetzung

- [x] 2.1 `testdata/fixtures/multi-turn-session.json` (synthetisch, nur erfundene Werte) und `testdata/fixtures/README.md`.
- [x] 2.2 Gegenprobe mit Platzhaltern ohne Kontext; Befund in `design.md` und als #97.
- [x] 2.3 `docs/phasen/phase-2.md` mit Anleitung für die manuelle Abnahme mit echtem Verkehr.

## 3. Abschluss

- [x] 3.1 `make lint`, `make test`, `make build`.
- [x] 3.2 `openspec validate issue-17-phase2-abnahme --strict`.
- [ ] 3.3 Manuelle Abnahme mit echtem Claude-Code-Verkehr laut `docs/phasen/phase-2.md` (Niclas).

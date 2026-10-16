# Gate 2
Branch `issue-91/negative-ersparnis`, gemeinsamer Workspace.

## Roter Vorlauf
Vor der Umsetzung `pnpm exec vitest run src/lib/format.test.ts` in `web/`: 7 von 12 Tests rot, alle Fälle von `formatSavedDelta` (Funktion existiert noch nicht, `formatSavedDelta is not a function`). Log lokal unter `/tmp/tollgate-issue91-red.log`.

## Validierung
- Nach Umsetzung `pnpm test`: 12 Dateien, 111 Tests grün (inkl. 7 Tabellenfälle `formatSavedDelta`).
- `pnpm check`: 0 Fehler, 0 Warnungen. `make lint` (`go vet ./...`) grün.
- `openspec validate issue-91-negative-ersparnis --strict` grün.
- Prüfung im gemeinsamen Workspace; kein Commit, Push oder PR.

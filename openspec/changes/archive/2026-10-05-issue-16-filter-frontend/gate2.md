# Gate 2

Branch `issue-16/filter-frontend`, gemeinsamer Workspace (geprüft wurde die Kombination aller
angewendeten Branches und der uncommitteten Arbeit von #83/#84).

Rot vor der Umsetzung (Typen und Signaturen als Stubs, `RecentRedactions` gibt nichts zurück,
`DryRunDraft` ignoriert `words_from`, keine Routen, keine Views):

- `internal/store`: `TestMatchPlaceholderGrammar` (Klartext im Platzhalterfeld angenommen),
  `TestRecentRedactions` (0 statt 10 Treffer).
- `internal/dashboard`: `TestTraceDetail`, `TestRecentRedactionsEndpoint` (404, keine Routen),
  `TestNoPlaintextInDetailAndRedactions` (keine Route; Redact-Stufe ersetzt erst nach dem
  Fixture-Wechsel auf eine nicht reservierte Domain).
- `internal/redactcfg`: `TestWordsFromVersion`, `TestWordsFromRejections`, `TestWordsFromAPI`
  (Wortliste nicht übernommen, maskierte Einträge angenommen),
  `TestFormAndYAMLGiveTheSameConfig` (Golden-Datei leer).
- `web/src/filters.test.ts`: alle 9 Tests (kein Bereich „Aktive Filter“, „Live-Test“,
  „Versionen“, „Import und Export“).
- `web/src/timeline.test.ts`: beide Tests (keine „Stufen-Timeline“).
- `web/src/home.test.ts`: die zwei neuen Tests (kein Bereich „Letzte Redactions“); die neun
  vorhandenen bleiben grün.

Grün nach der Umsetzung (2026-10-05):

- `go test -race ./...`: alle Pakete `ok` (u. a. `internal/store`, `internal/dashboard`,
  `internal/redactcfg` inklusive `TestFormAndYAMLGiveTheSameConfig`, `internal/redact`,
  `internal/server`, `internal/pipeline`, `cmd/tollgate`).
- `pnpm test`: 12 Dateien, 104 Tests grün; `pnpm check`: 0 Fehler, 0 Warnungen.
- `make lint` grün, `make generate` erzeugt bytegleiche Typen, `make web` und `make build` grün.
- Die Golden-Datei `web/src/testdata/form-config.yaml` ist die tatsächliche Formularausgabe; der
  Go-Test vergleicht ihren Export mit dem des handgeschriebenen YAML.
- Testanpassung: Im Export-Test wartet `createObjectURL` jetzt per `waitFor`, weil der Download
  erst nach dem asynchronen Lesen der Antwort entsteht. An der Erwartung ändert das nichts.
- Screenshots: `.claude/belege/issue16/{filter,timeline,home}-{desktop,iphone}-{hell,dunkel}.png`
  gegen `bin/tollgate` (127.0.0.1, `--dev-allow-any-listen`), Mock-Upstream, erfundene Daten.

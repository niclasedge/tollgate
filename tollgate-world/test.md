# Tests · Pruefung

## Auf einen Blick

`just test` runs the suite; `just lint` checks without autofix; `just cov` writes `coverage/lcov.info`. File arguments select test or lint targets.

`make test` ist die Projektprüfung: Frontend-Tests plus `go test -race ./...`; die CI führt sie ebenfalls aus. Getestet wird pro Stufe, mit Golden-Files, Fuzzing, Integrations-, Cache-, Konfigurations-, Paritäts- und End-to-End-Tests.

## Testarten

- Tabellengetriebene Unit-Tests pro Stufe, Golden-Files unter `testdata/` für jede
  Kompressionsregel.
- Fuzz-Tests mit `testing.F`: Redact und anschließendes Rehydrate ergeben exakt die
  Ausgangsdaten. Jede Transformation ist deterministisch.
- Integrationstests mit `httptest` als Mock-Upstream, inklusive SSE-Streams, bei denen
  Platzhalter über Chunk-Grenzen verteilt sind.
- Cache-Test: Mehrere Turns hintereinander erzeugen identische Präfixe, auch nach
  einer Konfigurationsänderung während einer laufenden Sitzung.
- Konfigurationstests: YAML-Import und -Export ergeben dieselbe Konfiguration,
  ungültiges YAML wird mit verständlicher Fehlermeldung abgelehnt.
- Paritätstest zwischen Rust- und Go-Redactor (siehe `CLAUDE.md`, „Redactor-Port nach Go“).
- End-to-End mit aufgezeichneten, bereinigten Fixtures aus `testdata/fixtures/`.
- `go test -race ./...` muss grün sein. Ein Befehl startet alle Tests inklusive
  Frontend (`make test`), dazu eine CI-Konfiguration.

Belege: ADR-0004 · ADR-0005

Wo: `Makefile`, `.github/workflows/ci.yml`, `testdata/fixtures/`, `internal/redact/testdata/`, `internal/redact/fuzz_test.go`, `cmd/tollgate/acceptance_test.go`, `web/src/`

## Standards

Was Tests prüfen müssen und was nicht erlaubt ist (löschen, abschwächen, skippen): `docs/standards.md`, Abschnitt „Verhalten prüfen“.

Wo: `docs/standards.md`

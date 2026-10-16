# Tasks

## 1. Aufzeichnen

- [x] 1.1 Anfrage von Claude Code 2.1.289 gegen lokalen Mock aufzeichnen, Quellen prüfen.
- [x] 1.2 Fixture und README mit Bereinigung ablegen.

## 2. Test

- [x] 2.1 `TestSessionKeyFromRecordedFixture` (Header; ohne Header Metadata).
- [x] 2.2 Kommentar an `sessionFromUserID` aktualisieren.

## 3. Prüfung

- [x] 3.1 `make lint`, `go test -race ./internal/pipeline/...`.
- [x] 3.2 `openspec validate --all --strict`.

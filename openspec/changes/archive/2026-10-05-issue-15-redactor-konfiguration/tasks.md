# Tasks

## 1. Tests zuerst

- [x] 1.1 `internal/redact/config_test.go`: eigene Regex (Treffer, Beispiel trifft nicht, leerer Treffer, Name doppelt, Platzhalter-Grammatik und Kollision), Wortliste (`case_sensitive`, `whole_word` mit Umlauten, Mehrwort mit Zeilenumbruch, Sortierung, kein Wort in Fehlern), Allowlist hat Vorrang, Determinismus.
- [x] 1.2 `internal/redactcfg/parse_test.go`: Roundtrip, Schema-Fehler mit Zeile und Spalte, kaputtes YAML, mehrere Dokumente, doppelte Schlüssel, Standardwerte, Anker.
- [x] 1.3 `internal/redactcfg/manager_test.go`: erster Start, Versionen mit Diff, Rollback, Hot Reload unter `-race`, Pinning über `Get`, ohne Sealer, Dry-Run, Leck-Test über DB, Logs und Antworten.
- [x] 1.4 `internal/redactcfg/api_test.go`: alle Endpunkte, 422 mit Issues, Export-Warnung.
- [x] 1.5 Rot sehen (Gate 2).

## 2. Umsetzung

- [x] 2.1 Schema `config/schema/tollgate-config.v1.schema.json` und `config/schema/schema.go`.
- [x] 2.2 `internal/redact/config.go`, `words.go`, Hooks in `rules.go` und `redact.go`.
- [x] 2.3 `internal/redactcfg`: Modell, Parse, Export, Diff, Manager, Dry-Run, API.
- [x] 2.4 Synthetische Fixture und `fixtures.go` (Einbettung).
- [x] 2.5 `cmd/tollgate/main.go`: Manager öffnen, Routen registrieren.

## 3. Abschluss

- [x] 3.1 `go vet ./...`, `make lint`, `make test`, Paritätstest `go test ./internal/redact/...`.
- [x] 3.2 `openspec validate issue-15-redactor-konfiguration --strict`.

## 4. Offen

- [ ] 4.1 Nach Merge von #28: `*cryptobox.Box` als `Sealer` in `cmd/tollgate/main.go` übergeben (Schlüssel aus `crypto.key_file`, auch ohne Management-Key öffnen).
- [ ] 4.2 Mit #13: `Manager.Redactor` als `redact.Resolver` an die Stufe übergeben.

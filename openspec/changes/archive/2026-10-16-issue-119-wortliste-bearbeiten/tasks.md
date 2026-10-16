# Tasks

## 1. Backend (`internal/redactcfg`)

- [x] 1.1 Tests zuerst: `wordops_test.go` tabellengetrieben (Hinzufügen, Entfernen, Dubletten mit/ohne `case_sensitive`, leere Einträge, Ändern, ungültiger/doppelter Index, Grenzen, Maskierungsfüller, Determinismus, Schemagrenzen gleich); API-Tests (400/422, Import über `words_from` + Operationen); rot sehen.
- [x] 1.2 `wordops.go`: `applyWordOps`, Faltungsschlüssel, `WordOpError`; `WordOps` in `api.go` einbetten, Fehler-Mapping 400.
- [x] 1.3 `manager.go`: Operationen in `Draft`, `ImportRequest`, `TestRequest`, `parseDraft`.
- [x] 1.4 Leck-Test erweitern: kein Klartext in Antworten, Log und DB-Dateien nach Operationen.
- [x] 1.5 `make generate` (tygo).

## 2. Frontend

- [x] 2.1 Tests zuerst (Vitest): Hinzufügen und Entfernen erzeugen die richtigen Requests, Liste bleibt maskiert, Operation sperrt Speichern; rot sehen.
- [x] 2.2 `api.ts`: optionaler Parameter `ops` an `dryRun`, `importConfig`, `testFilter`.
- [x] 2.3 `filterdraft.ts`: `splitWords`, `removeWord`.
- [x] 2.4 `Filters.svelte`: Feld „Wörter hinzufügen“, „Entfernen“ je Eintrag, Zustand im Dry-Run-Schnappschuss.

## 3. Abschluss

- [x] 3.1 `go test -race ./internal/redactcfg/...`, `cd web && pnpm check && pnpm vitest run`, `make lint`.
- [x] 3.2 `openspec validate issue-119-wortliste-bearbeiten --strict`.

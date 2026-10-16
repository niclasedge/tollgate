# Tasks

## 1. Redactor (`internal/redact`)

- [x] 1.1 Tests zuerst: Kunde/Vorname/Nachname-Typen, längster Eintrag gewinnt, Reihenfolge der Einträge ohne Einfluss, Allowlist, Numerierung je Typ, Maske nur Länge, Rückweg im Stream über Chunk-Grenzen, Fuzz-Seeds für Redact → Rehydrate; rot sehen.
- [x] 1.2 `config.go`/`words.go`/`mask.go`: `WordList.Customers/FirstNames/LastNames`, Typkonstanten, `IsListType`, `wordRule` mit Typ und Regelname.

## 2. Konfiguration (`internal/redactcfg`, Schema)

- [x] 2.1 Tests zuerst: Round-Trip aller vier Listen (omit/plain/maskiert), altes YAML ohne neue Schlüssel, Schemagrenzen gleich, altes Blob-Format, Klartext-Endpunkt (401/404/422/no-store), `lists_set` (Ersetzen, Trimmen, Dubletten, Grenzen, Maskierungsfüller, `words_from`-Konflikt), Leck-Test, gepinnte Sitzung; rot sehen.
- [x] 2.2 `model.go`, `parse.go`, Schema: neue Felder, Export, Klon, Normalisierung, Maskenprüfung.
- [x] 2.3 `lists.go`, `wordops.go`: gemeinsames Säubern, `lists_set`; `manager.go`: `Draft`/`ImportRequest`/`TestRequest`, `parseDraft`, versiegeltes Objekt (beide Formate), `PlainLists`; `api.go`: Request-Felder, Route `GET /api/config/lists`.
- [x] 2.4 `make generate` (tygo).

## 3. Frontend

- [x] 3.1 Tests zuerst (Vitest): maskierte Abschnitte ohne Abruf, „Bearbeiten“ lädt einmal, Zeilen/Dubletten, `lists_set` an Dry-Run und Import, Änderung sperrt Speichern, „Wortliste ersetzen“ erhält Listen; rot sehen.
- [x] 3.2 `api.ts`, `filterdraft.ts`, `NameLists.svelte`, `Filters.svelte`.

## 4. Doku und Abschluss

- [x] 4.1 ADR-0021 und Index; Regel in `CLAUDE.md` (Wortliste maskiert → Ausnahme ADR-0021); `docs/adr/0008` unverändert (ADR wird nicht umgeschrieben), Verweis im ADR-0021.
- [x] 4.2 `make test`, `make lint`, `cd web && pnpm exec svelte-check`.
- [x] 4.3 `openspec validate issue-143-namenslisten --strict`.
- [x] 4.4 Deploy auf den Mac mini (Koordinator).

# Tasks

## 1. Tests zuerst

- [x] 1.1 `internal/store/redactions_test.go`: Platzhalter-Grammatik (Klartext abgelehnt), `RecentRedactions` mit Reihenfolge, Limit und Client-Name; Feld-Allowlist in `store_test.go` bewusst um `Placeholder` erweitert.
- [x] 1.2 `internal/dashboard/detail_test.go`: `GET /api/traces/{id}` (Stufen, Diff, Treffer, 404), `GET /api/redactions` (Limit, Reihenfolge, 400), Admin-Token, kein Klartext nach einer echten Anfrage durch Server und Redact-Stufe.
- [x] 1.3 `internal/redactcfg/words_test.go`: `words_from` in Dry-Run, Import und Test; maskierte Einträge abgelehnt; Wortliste plus `words_from` abgelehnt; unbekannte Version 404.
- [x] 1.4 `internal/redactcfg/equivalence_test.go`: `web/src/testdata/form-config.yaml` und handgeschriebenes YAML ergeben bytegleiche Exporte.
- [x] 1.5 `web/src/filters.test.ts`: Liste, maskierte Wortliste, Formular erzeugt `form-config.yaml`, Speichern erst nach grünem Dry-Run, Fehler mit Zeile/Spalte, `words_from`, Live-Test, Versionen mit Diff und Rollback, Export-Warnung.
- [x] 1.6 `web/src/timeline.test.ts`: Timeline in Live mit Stufen, Diff und Rückweg-Zeile; `web/src/home.test.ts`: Panel „Letzte Redactions“.
- [x] 1.7 Rot sehen (Gate 2).

## 2. Umsetzung

- [x] 2.1 `Placeholder` in `pipeline.Match`, `redact.Number`, `server.recordTrace`, `store.Match` mit Validierung; `store.RecentRedactions`.
- [x] 2.2 `internal/dashboard`: Typen, `GET /api/traces/{id}`, `GET /api/redactions`.
- [x] 2.3 `internal/redactcfg`: `words_from`, Masken-Prüfung.
- [x] 2.4 `tygo.yaml` um `internal/redactcfg` ergänzen, `make generate`.
- [x] 2.5 Frontend: Abhängigkeiten (pnpm), `api.ts`, `filterdraft.ts`, `YamlEditor.svelte`, `Filters.svelte`, `Timeline.svelte` in Live, Panel in Home, Route in `App.svelte`.

## 3. Abschluss

- [x] 3.1 `make lint`, `make test`, `make web build`.
- [x] 3.2 Screenshots unter `.claude/belege/issue16/` (Desktop und iPhone-Breite, hell und dunkel).
- [x] 3.3 `openspec validate issue-16-filter-frontend --strict`.

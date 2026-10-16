# Proposal

Issue: [#16 Frontend Phase 2: Filter-Bereich und Anfrage-Timeline](https://github.com/niclasedge/tollgate/issues/16)

Depends on #10 (Frontend Phase 1), #14 (Rückweg), #15 (Redactor-Konfiguration) und #76
(Übersicht). Verbindlich sind [ADR-0004](../../../docs/adr/0004-json-byte-splicing-und-determinismus.md)
(Diff aus Ersetzungslisten) und [ADR-0008](../../../docs/adr/0008-konfigurationsschema.md)
(Live-Validierung gegen das ausgelieferte Schema, Dry-Run vor dem Aktivieren).

## Why

Seit #15 ist die Redactor-Konfiguration versioniert und über `/api/config/*` pflegbar, aber
nur per `curl`. Die gespeicherten Stufen-Messwerte (Dauer, Tokens, Hashes, Diff, Treffer)
sind im Frontend unsichtbar. Der Auftrag verlangt einen Filter-Bereich, eine Stufen-Timeline
pro Anfrage und (Ergänzung 2026-10-04) die letzten 10 Redactions auf der Startseite.

## What Changes

- **Filter-Bereich** (`web/src/lib/views/Filters.svelte`): Liste der aktiven Filter
  (Schalter, eigene Regeln, maskierte Wortliste, Allowlist) als Formular, CodeMirror-YAML-
  Editor mit Live-Validierung gegen `GET /api/config/schema`, Dry-Run-Vorschau vor dem
  Speichern, Live-Test, Versionshistorie mit Diff, Aktivieren und Rollback, Import aus Datei
  und Export (mit Wortliste nur nach Warnung). Formular und Editor bearbeiten denselben
  YAML-Entwurf und speichern über denselben Import-Endpunkt.
- **Wortliste übernehmen** (`internal/redactcfg`): `dry-run`, `import` und `test` nehmen
  `words_from` (Versions-ID). Dann gilt die gespeicherte Wortliste dieser Version, ohne dass
  sie den Browser im Klartext erreicht. Einträge mit dem Maskierungsfüller `••••••••` werden
  abgelehnt, damit eine maskierte Fassung nie als Wortliste gespeichert wird.
- **Platzhalter in Treffern** (`internal/pipeline`, `internal/redact`, `internal/server`,
  `internal/store`): `Match` bekommt das Feld `Placeholder` (`<redacted:TYP_N>`). Der Store
  prüft es gegen die Platzhalter-Grammatik aus ADR-0006, sodass dort kein Klartext landen kann.
- **Dashboard-API** (`internal/dashboard`): `GET /api/traces/{id}` liefert die Anfrage mit
  allen Stufen (Dauer, Bytes, Tokens vorher/nachher, Hashes, Konfigurationsversion, Diff,
  Treffer). `GET /api/redactions?limit=` liefert die letzten Treffer (Zeit, Client, Typ,
  Platzhalter, Regel, Trace-ID), nie Werte.
- **Timeline in Live**: Die inline aufklappende Detailansicht lädt die Stufen und zeigt sie
  als Timeline mit Diff pro Stufe; der Rückweg ist eine eigene Zeile.
- **Übersicht**: Panel „Letzte Redactions“ mit Link ins Live-Detail.
- **Typen**: `tygo.yaml` erzeugt zusätzlich `web/src/lib/types/redactcfg.ts`.

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `dashboard-api`: Trace-Detail und letzte Redactions.
- `store`: Platzhalter in Treffern, Abfrage der letzten Treffer.
- `redaction-config`: Wortliste aus einer Version übernehmen, maskierte Einträge abgelehnt.
- `frontend-views`: Filter-Bereich, Stufen-Timeline, letzte Redactions.

## Impact

Branch `issue-16/filter-frontend`, gemeinsamer Workspace. Berührte Dateien:

- Go: `internal/pipeline/pipeline.go`, `internal/redact/stage.go`, `internal/server/server.go`,
  `internal/store/traces.go`, `internal/store/redactions.go` (neu),
  `internal/store/store_test.go` (Feld-Allowlist), `internal/store/redactions_test.go` (neu),
  `internal/dashboard/types.go`, `internal/dashboard/handler.go`, `internal/dashboard/convert.go`,
  `internal/dashboard/detail.go` (neu), `internal/dashboard/detail_test.go` (neu),
  `internal/redactcfg/api.go`, `internal/redactcfg/manager.go`, `internal/redactcfg/dryrun.go`,
  `internal/redactcfg/parse.go`, `internal/redactcfg/words_test.go` (neu),
  `internal/redactcfg/equivalence_test.go` (neu), `tygo.yaml`.
- Frontend: `web/package.json`, `web/pnpm-lock.yaml` (CodeMirror 6, `@codemirror/lang-yaml`,
  `codemirror-json-schema`, `yaml` laut ADR-0003), `web/src/App.svelte`, `web/src/lib/api.ts`,
  `web/src/lib/views/Filters.svelte` (neu), `web/src/lib/views/Live.svelte`,
  `web/src/lib/views/Home.svelte`, `web/src/lib/components/YamlEditor.svelte` (neu),
  `web/src/lib/components/Timeline.svelte` (neu), `web/src/lib/filterdraft.ts` (neu),
  `web/src/lib/types/redactcfg.ts` (generiert), `web/src/lib/types/dashboard.ts` (generiert),
  `web/src/filters.test.ts` (neu), `web/src/timeline.test.ts` (neu), `web/src/home.test.ts`,
  `web/src/test-helpers.ts`, `web/src/testdata/form-config.yaml` (neu).
- Keine Migration (Treffer liegen als JSON in `trace_stages.matches`), keine Änderung am
  weitergeleiteten Body.

## Was ausdrücklich nicht

- Kein Kompressions-Playground (#21) und keine Bearbeitung des Abschnitts `compress:` im
  Formular; der Abschnitt bleibt im YAML erhalten.
- Kein eigener Bereich „Anfragen“: die Timeline klappt in Live inline auf.
- Keine Rückübersetzungs-Zählung im Store; die Rückweg-Zeile zeigt Antwortstatus, Dauer,
  Ausgabe-Tokens und die Zahl der rückübersetzbaren Platzhalter aus den Treffern.

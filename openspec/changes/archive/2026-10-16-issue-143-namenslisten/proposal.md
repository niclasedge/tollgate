# Proposal

Issue: [#143 Filter: eigene Listen für Kunden, Vornamen und Nachnamen](https://github.com/niclasedge/tollgate/issues/143)

## Why

Im Filter-Bereich gibt es nur eine allgemeine Wortliste mit dem Platzhalter `word`. Wer
Kunden, Vornamen und Nachnamen getrennt pflegen will, kann das nicht, und im Verlauf lässt
sich nicht unterscheiden, ob `<redacted:word_3>` ein Kunde oder ein Vorname war. Außerdem
lässt sich die Wortliste nur maskiert ansehen; „ändern“ heißt entfernen und neu eintippen.
Für Namenslisten mit vielen Zeilen ist ein Textfeld mit der kompletten Liste nötig.

## What Changes

- **Drei neue vertrauliche Listen** `words.customers` (Platzhalter `kunde`),
  `words.first_names` (`vorname`) und `words.last_names` (`nachname`), nummeriert und
  deterministisch (`<redacted:kunde_1>` …). Sie teilen `case_sensitive` und `whole_word`
  mit der Wortliste und haben dieselbe Treffersemantik (längste zuerst, Allowlist,
  Unicode-Wortgrenzen).
- **`words.list` bleibt** als allgemeine Liste mit Platzhalter `word`. Keine Migration:
  alte YAML-Dateien und gespeicherte Versionen bleiben gültig.
- **Speicherung:** Der versiegelte Blob enthält ein Objekt mit allen vier Listen; das alte
  Format (JSON-Array = nur `words.list`) wird weiter gelesen.
- **YAML und Schema:** Erweiterung des Blocks `words`; Import, Export (`omit`/`plain`/maskiert),
  JSON-Schema und Round-Trip.
- **Entwurfs-Operation `lists_set`:** ersetzt einzelne Listen (Kunden, Vornamen,
  Nachnamen) vollständig, eine fehlende Liste bleibt; serverseitig angewendet (Trimmen, leere Zeilen und Dubletten verwerfen,
  Schemagrenzen), neben `words_from`/`words_add`/`words_remove`.
- **Klartext-Endpunkt `GET /api/config/lists`** (nur Admin, `no-store`): liefert die drei
  Listen einer Version im Klartext, damit „Bearbeiten“ sie in ein Textfeld laden kann.
  Neue, eng begrenzte Ausnahme der Regel „Wortliste maskiert anzeigen“: ADR-0021.
- **Frontend:** drei Abschnitte Kunden, Vornamen, Nachnamen mit maskierten Einträgen und
  Anzahl. „Bearbeiten“ lädt den Klartext der Liste einmal in ein Textfeld (eine Zeile je Eintrag),
  Speichern läuft über Entwurf → Prüfen → Dry-Run → Aktivieren.
- **Aufdecken (ADR-0018) bleibt aus** für die drei neuen Typen, wie für `word`.
- **Neue Regeln gelten nur für neue Sitzungen** (bestehender Versions-Pin je Sitzung).

## Capabilities

### Modified Capabilities

- `redaction`: drei weitere Wortlisten-Typen mit eigenen Platzhaltern und Masken.
- `redaction-config`: Schema, Import/Export, Versiegelung, `lists_set`, Klartext-Endpunkt.
- `frontend-views`: Filter-Bereich mit drei Listen und Textfeld-Bearbeitung.

## Impact

- Neu: `docs/adr/0021-namenslisten-klartext-bearbeiten.md`,
  `internal/redactcfg/lists.go`, `internal/redactcfg/lists_test.go`,
  `web/src/lib/components/NameLists.svelte`, diese Change-Artefakte.
- Eigene Stellen in: `internal/redact/config.go`, `words.go`, `mask.go` (+ Tests),
  `internal/redactcfg/model.go`, `parse.go`, `manager.go`, `api.go`, `dryrun.go`,
  `wordops.go` (+ Tests), `config/schema/tollgate-config.v1.schema.json`,
  `web/src/lib/types/redactcfg.ts` (generiert), `web/src/lib/api.ts`,
  `web/src/lib/filterdraft.ts`, `web/src/lib/views/Filters.svelte`,
  `web/src/filters.test.ts`, `docs/adr/README.md`, `CLAUDE.md` (eine Zeile der Regel).
- Kein Eingriff in fremde Changes. Nicht angefasst, aber betroffen: `internal/reveal` und
  `web/src/lib/components/RevealMask.svelte` (Issue #114) schließen `word` und
  `private_key` vom Aufdecken aus; die neuen Typen gehören dort ebenfalls hinein
  (Hinweis an den Koordinator, `redact.IsListType` steht bereit).

## Was ausdrücklich nicht

- Keine Migration von `words.list` in die neuen Listen.
- Kein Klartext der allgemeinen Wortliste über den neuen Endpunkt (sie bleibt maskiert,
  Bearbeiten wie bisher über `words_add`/`words_remove`).
- Kein Klartext im Log, in der DB oder im Trace; nur der explizite Klick holt die Listen.

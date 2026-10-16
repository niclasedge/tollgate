# Proposal

Issue: [#119 Wortliste: Wörter hinzufügen und einzeln entfernen (Klartext bleibt verdeckt)](https://github.com/niclasedge/tollgate/issues/119)

## Why

Die Wortliste lässt sich im Filter-Bereich heute nur als Ganzes ersetzen („Wortliste
ersetzen“). Wer ein Wort ergänzen oder einen Eintrag streichen will, muss die gesamte Liste
neu eingeben, obwohl das Frontend sie nur maskiert kennt. Das ist umständlich und
fehleranfällig.

## What Changes

- **Operationen statt Klartext:** `POST /api/config/dry-run`, `/import` und `/test`
  nehmen zusätzlich `words_remove` (Indizes der Basisliste) und `words_add` (neue Wörter)
  an. Basisliste ist die Wortliste aus `words_from` oder, ohne `words_from`, die Liste im
  Entwurf. Der Server wendet die Operationen auf die entschlüsselte Basisliste an; ihr
  Klartext verlässt den Server nie.
- **Deterministisch:** Ergebnis = Basisliste ohne entfernte Indizes, dann neue Wörter in
  Eingabereihenfolge. Neue Wörter werden getrimmt; leere Einträge und Dubletten (gemäß
  `case_sensitive`) werden verworfen.
- **Fehler:** Ungültiger oder doppelter Index → 400 mit verständlicher Meldung, die nur
  Indizes nennt. Neue Wörter, die Schemagrenzen verletzen oder den Maskierungsfüller
  enthalten → 422 mit Issue an `words_add[i]`, ohne den Eintrag zu zitieren.
- **Frontend:** Abschnitt Wortliste bekommt das Feld „Wörter hinzufügen“ (ein Eintrag je
  Zeile oder kommagetrennt) und „Entfernen“ an jedem maskierten Eintrag. „Wortliste
  ersetzen“ bleibt. Danach wie bisher Prüfen (Dry-Run) → Speichern.

## Capabilities

### Modified Capabilities

- `redaction-config`: Wortlisten-Operationen `words_remove`/`words_add` an Dry-Run, Import
  und Live-Test.
- `frontend-views`: Filter-Bereich mit „Wörter hinzufügen“ und „Entfernen“ je Eintrag.

## Impact

- Neu: `internal/redactcfg/wordops.go`, `internal/redactcfg/wordops_test.go`, diese
  Change-Artefakte.
- Eigene Stellen in: `internal/redactcfg/api.go` (Typ `WordOps`, Request-Bodies, Fehler-
  Mapping), `internal/redactcfg/manager.go` (`Draft`, `ImportRequest`, `TestRequest`,
  `parseDraft`), `internal/redactcfg/manager_test.go` (Leck-Test),
  `web/src/lib/types/redactcfg.ts` (generiert), `web/src/lib/api.ts` (drei Config-
  Aufrufe), `web/src/lib/filterdraft.ts` (`removeWord`, `splitWords`),
  `web/src/lib/views/Filters.svelte` (Abschnitt Wortliste), `web/src/filters.test.ts`.
- Kein Schema-Wechsel: `config/schema/` beschreibt das YAML-Dokument, die Operationen sind
  Felder der Anfrage. Kein ADR-Bruch (ADR-0009 bleibt: Wortliste nur verschlüsselt,
  maskiert angezeigt, nie geloggt).

## Was ausdrücklich nicht

- Kein Aufdecken oder Bearbeiten im Klartext; „Ändern“ heißt Entfernen und neu Hinzufügen.
- Keine Bereinigung bestehender Dubletten in der Basisliste.
- Keine neue Endpunkt-Route; die Operationen hängen am bestehenden Entwurfs-Weg.

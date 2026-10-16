# Design

Grundlage: ADR-0008 (Entwurf → Dry-Run → Version), ADR-0009 (verschlüsselte Wortliste,
maskierte Anzeige), der bestehende `words_from`-Mechanismus und AGENTS.md (Determinismus,
nur redigierte Daten in Log, DB und Frontend).

## 1. Operationen am Entwurf

Neuer API-Typ, eingebettet in `DryRunRequest`, `ImportBody` und `TestBody`:

```go
type WordOps struct {
	WordsRemove []int    `json:"words_remove,omitempty"`
	WordsAdd    []string `json:"words_add,omitempty"`
}
```

`Draft`, `ImportRequest` und `TestRequest` tragen dieselben Felder. `parseDraft` wendet sie
nach dem Parsen und nach `words_from` an:

1. Basisliste = Liste aus `words_from` (wenn gesetzt) sonst die Liste im Entwurf.
2. Jeder Index in `words_remove` muss in `[0, len(Basis))` liegen und darf nur einmal
   vorkommen, sonst `*WordOpError` (Status 400). Die Reihenfolge der Indizes ist egal.
3. Ergebnis = Basis ohne die entfernten Indizes (Reihenfolge der Basis bleibt).
4. Jedes Wort aus `words_add` wird mit `strings.TrimSpace` getrimmt. Leere Einträge
   entfallen. Ein Wort, das (gemäß `case_sensitive`) schon im Ergebnis steht, entfällt
   ebenfalls; sonst wird es angehängt. Dadurch funktioniert „Ändern“ als Entfernen plus
   neu Hinzufügen desselben Worts.
5. Danach Prüfung wie beim YAML: Länge höchstens 200 Zeichen, höchstens 5000 Einträge
   (Grenzen aus `config/schema/`, ein Test hält sie gleich), kein Maskierungsfüller. Fehler
   sind `*ValidationError` mit Pfad `words_add[i]` (bzw. `words_add` für die Gesamtzahl),
   ohne Zeile/Spalte und ohne den Eintrag zu zitieren → 422.
6. `normalize`, `redact.Compile`.

Ohne Operationen und ohne `words_from` bleibt der Weg unverändert.

### Dubletten ohne Groß-/Kleinschreibung

Der Redactor matcht mit `(?i)` (einfache Unicode-Faltung). Der Vergleichsschlüssel faltet
daher jede Rune auf das kleinste Element ihrer `unicode.SimpleFold`-Bahn; das entspricht
`strings.EqualFold`, ist aber als Map-Schlüssel nutzbar (O(n)). Bei `case_sensitive: true`
ist der Schlüssel das getrimmte Wort selbst.

## 2. Vertraulichkeit

- Antworten: Dry-Run und Test liefern wie bisher nur Treffer ohne Werte; Import liefert
  `VersionInfo` mit maskiertem Diff (`create` diffed die maskierten Exporte).
- Fehler: `WordOpError` nennt nur Indizes und die Länge der Basis; Issues an `words_add[i]`
  nennen nur Position und Grund.
- Log: `fail` loggt nur unbekannte Fehler; `WordOpError` und `ValidationError` werden nicht
  geloggt.
- DB: Inhalt ohne Wortliste, Wortliste versiegelt (unverändert).

## 3. Frontend

Zusätzlicher Zustand neben `wordsFrom`: `wordsRemove: number[]` (Indizes in die
übernommene Liste) und `wordsAdd: string[]`. Beide gehören zum Dry-Run-Schnappschuss;
jede Änderung macht ihn ungültig.

- **Hinzufügen:** Feld „Wörter hinzufügen“ trennt an Zeilenumbruch und Komma, trimmt und
  hängt an `wordsAdd` an. Angezeigt werden neue Einträge ebenfalls nur maskiert.
- **Entfernen:** Bei `wordsFrom > 0` landet der Index in `wordsRemove`, der Eintrag
  verschwindet aus der Anzeige. Ohne `wordsFrom` steht die Liste im Entwurf (vom Nutzer
  selbst eingegeben oder aus einer Datei); dann löscht `removeWord` den Eintrag direkt im
  YAML. So verweisen Indizes in `words_remove` nur auf eine gespeicherte, unveränderliche
  Version. Neue, noch nicht gespeicherte Einträge lassen sich aus `wordsAdd` entfernen.
- Ersetzen, Wortliste entfernen, Version laden und Datei laden setzen beide Listen zurück.
- `api.dryRun`, `api.importConfig` und `api.testFilter` bekommen einen optionalen Parameter
  `ops: WordOps` (generierter Typ), der nur nicht-leere Felder sendet.

## 4. Alternativen

- **Neue Route `PATCH /api/config/words`:** würde einen zweiten Weg neben Dry-Run → Import
  schaffen und den Dry-Run umgehen. Verworfen.
- **Klartext-Wortliste ans Frontend:** bricht ADR-0009. Verworfen.
- **Wörter per Hash adressieren statt Index:** würde Hashes vertraulicher Wörter
  herausgeben (Wörterbuchangriff). Index auf eine unveränderliche Version ist eindeutig und
  verrät nichts.

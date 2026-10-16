# Design

Grundlage: ADR-0005 (Redactor), ADR-0006 (Platzhalter, Sitzungen), ADR-0008 (Schema,
Versionen), ADR-0009 (Verschlüsselung der Wortliste), ADR-0018 (Aufdecken als schmale
Ausnahme), #119 (Operationen am Entwurf).

## 1. YAML und Schema

```yaml
words:
  case_sensitive: false
  whole_word: true
  list: ["Projekt Falke"]        # allgemein, Platzhalter word
  customers: ["Muster GmbH"]     # Platzhalter kunde
  first_names: ["Anna"]          # Platzhalter vorname
  last_names: ["Müller"]         # Platzhalter nachname
```

Die drei Listen teilen `case_sensitive`/`whole_word`. Jede hat dieselben Grenzen wie
`list` (höchstens 5000 Einträge, je 1 bis 200 Zeichen). Fehlt ein Schlüssel, ist die Liste
leer. Das Schema dupliziert den Block je Liste (kein `$ref`), damit der Editor-Linter im
Frontend ihn ohne Auflösung versteht; ein Test hält die Grenzen gleich.

Export schreibt immer alle vier Listen aus: `omit` als `[]`, `plain` im Klartext, maskiert
mit `Mask`. Alte Versionen (ohne die neuen Schlüssel) lesen sich als leere Listen, ihr
Export gewinnt nur die Zeilen `customers: []` usw. Weil der Diff aus zwei Exporten
derselben Parser-Ausgabe entsteht, entsteht dadurch kein Schein-Diff.

## 2. Redaction

`redact.WordList` bekommt `Customers`, `FirstNames`, `LastNames`. `Compile` baut je Eintrag
dieselbe Regel wie für `List` (`wordRule`), nur mit eigenem Platzhalter-Typ und Regelnamen
(`customers`, `first_names`, `last_names`; der Name verrät die Liste, nie den Eintrag).
Jede Liste wird für sich dedupliziert und nach Byte-Länge absteigend, dann lexikografisch
sortiert (`sortedWords`). Alle Regeln haben Priorität `prioWords`; bei Gleichstand von
Priorität, Länge und Start entscheidet die feste Reihenfolge der Regeln:
Wortliste, Kunden, Vornamen, Nachnamen. Längere Treffer gewinnen ohnehin zuerst, ein
Kunde „Anna Müller GmbH“ schlägt also den Vornamen „Anna“.

Die Nummerierung je Typ und die Rückübersetzung (auch in SSE-Chunks und
`input_json_delta`) hängen nicht am Typ; sie bleiben unverändert. Tests belegen es mit
den neuen Typen.

Die Typen `kunde`, `vorname`, `nachname` werden **nicht** als reserviert für eigene Regeln
erklärt: bestehende Konfigurationen mit einer eigenen Regel dieses Typs müssen gültig
bleiben (keine Migration). Ein gemeinsamer Typ ist unschädlich, weil die Nummerierung nach
(Typ, Wert) läuft. Die Maske dagegen behandelt die drei Typen wie `word`: nur die Länge
(„••• (N)“), kein erstes Zeichen. `redact.IsListType` nennt diese Typen für andere Pakete.

## 3. Versiegelter Blob

```go
type sealedWords struct {
	List       []string `json:"list"`
	Customers  []string `json:"customers"`
	FirstNames []string `json:"first_names"`
	LastNames  []string `json:"last_names"`
}
```

`create` versiegelt das Objekt, sobald irgendeine Liste Einträge hat. `load` liest beide
Formate: beginnt der Klartext mit `[`, ist es das alte Array (nur `list`). Ein Test hält
ein im alten Format versiegeltes Fixture lesbar.

## 4. Entwurf: `lists_set`

```go
type ListsSet struct {
	Customers  *[]string `json:"customers,omitempty"`
	FirstNames *[]string `json:"first_names,omitempty"`
	LastNames  *[]string `json:"last_names,omitempty"`
}
// in WordOps:
ListsSet *ListsSet `json:"lists_set,omitempty"`
```

Jede **vorhandene** Liste in `lists_set` ersetzt die Liste des Entwurfs (bzw. der
`words_from`-Version) vollständig, ein leeres Array leert sie; eine fehlende Liste bleibt
unverändert. So muss das Frontend nur die Liste im Klartext kennen, die der Nutzer
bearbeitet (Abweichung von der ersten Skizze „alle drei auf einmal“: weniger Klartext im
Browser). Ablauf in `parseDraft`:

1. `words_from > 0`: übernimmt alle vier Listen der Version; der Entwurf darf dann keine
   der vier Listen enthalten (Fehler am ersten belegten Pfad).
2. `words_remove`/`words_add` wirken auf `words.list` (wie #119).
3. `lists_set` ersetzt die vorhandenen Listen. Jeder Eintrag wird getrimmt, leere Einträge
   und Dubletten (gemäß `case_sensitive`, gleiche Faltung wie #119) entfallen, die
   Reihenfolge der Eingabe bleibt. Grenzen wie im Schema, kein Maskierungsfüller;
   Verstöße sind Issues an `lists_set.customers[i]` (ohne Zitat, 422).
4. `normalize`, `redact.Compile`.

Der Prüfschritt je Eintrag (`entryProblem`) wird von `applyWordOps` und `applyListsSet`
gemeinsam genutzt.

Ohne Operationen und ohne `words_from` bleibt der Weg unverändert.

## 5. Klartext-Endpunkt

`GET /api/config/lists?version=N` (ohne `version` die aktive) hinter dem Admin-Token
liefert `NameLists` (`customers`, `first_names`, `last_names`, nie `null`) der Version im
Klartext. Header `Cache-Control: no-store`. Fehler:
404 unbekannte Version, 422 ohne Schlüssel (`ErrNoSealer`), 400 ungültige Zahl. Geloggt
wird nur `version`, nie ein Wert. Die allgemeine Wortliste ist nicht Teil der Antwort.
Der Frontend-Aufruf passiert nur auf den Klick „Bearbeiten“, einmal je Versionsstand
(Ergebnis nur im Komponentenzustand, nie in `localStorage` oder im YAML-Entwurf).
Details und Abwägung: ADR-0021.

## 6. Frontend

Der Entwurfs-YAML bleibt die eine Wahrheit; für die drei Listen gilt dasselbe Muster wie
für die Wortliste:

- `draftFromVersion` leert alle vier Listen im Entwurf und liefert ihre Masken aus dem
  maskierten YAML der Version (`wordsFrom` nimmt sie über `words_from` mit).
- Neue Komponente `NameLists.svelte` zeigt je Liste Titel, Anzahl und maskierte Chips
  (`mask()`) und je Abschnitt „Bearbeiten“. „Bearbeiten“ öffnet das Textfeld dieser einen
  Liste (eine Zeile je Eintrag); der Klartext kommt bei `wordsFrom > 0` einmal von
  `GET /api/config/lists` (nur die gewählte Liste wird behalten), sonst aus dem
  Entwurf-YAML (der Nutzer hat ihn selbst dort) oder aus einer noch nicht gespeicherten
  Ersetzung. „Übernehmen“ trimmt, wirft leere Zeilen und Dubletten (gemäß
  `case_sensitive`) weg; „Abbrechen“ und Escape verwerfen das Textfeld samt Klartext.
- Bei `wordsFrom > 0` landet das Ergebnis im Zustand `listsSet` (nur bearbeitete Listen)
  und wird als `lists_set` gesendet (Entwurf bleibt leer); ohne `words_from` schreibt es
  direkt ins YAML.
- Jede Änderung macht den Dry-Run ungültig (wie `words_remove`/`words_add`).
- „Wortliste ersetzen“ und „Wortliste entfernen“ verwerfen bisher `words_from`. Hätte die
  Version Kunden-/Namenslisten, gingen die mit verloren; dann arbeiten beide Aktionen über
  `words_remove`/`words_add` auf der allgemeinen Liste und lassen `words_from` stehen. Ohne
  solche Listen bleibt das alte Verhalten (kein Testbruch).
- Tastatur: Textfeld, „Übernehmen“, „Abbrechen“ sind normale Bedienelemente mit sichtbarem
  Fokus; Dark Mode und Layout über die vorhandenen Variablen, einspaltig unter 900 px.
- Typen kommen aus tygo (`NameLists`, `WordOps.lists_set`); keine Handpflege.

## 7. Sitzungen und Cache

Nichts Neues: jede Sitzung hält ihre Konfigurationsversion (`Manager.Get` mit Pin).
Ein Test importiert eine Version mit `customers`, aktiviert sie, und zeigt, dass eine
auf die alte Version gepinnte Sitzung weiter ohne die neue Regel läuft, eine neue sie
anwendet.

## 8. Vertraulichkeit

- DB: nur versiegelt (AAD wie bisher), Content ohne Listen.
- Diff, Version, Export `omit`/maskiert, Fehlermeldungen, Log, Trace: keine Einträge.
  Der Leck-Test (`TestWordListLeak`) deckt die neuen Listen ab, der Mask-Test die Typen.
- Klartext nur: Export `words=plain` (Warnung wie bisher) und der neue Endpunkt.

## 9. Alternativen

- **Klartext der neuen Listen immer mitliefern:** landet in jeder Versionsantwort und im
  Browser-Cache. Verworfen.
- **Die allgemeine Liste mit in den Endpunkt nehmen:** widerspräche der Entscheidung, sie
  unverändert zu lassen. Verworfen.
- **Pro Liste ein eigener Endpunkt oder Operationen je Liste:** mehr Fläche, die Liste
  wird ohnehin als Ganzes bearbeitet. Verworfen.
- **Eigene Regeln mit den neuen Typen verbieten:** bricht bestehende Konfigurationen.
  Verworfen.

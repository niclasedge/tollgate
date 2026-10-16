# Design

## Tageswerte

- `daySpans(buckets)` teilt die Stundenachse in UTC-Tage (Indexbereiche, angeschnittene Tage
  eingeschlossen).
- `dayBands(buckets, x, width)` liefert je Tag die Trennlinie vor dem Tag und die Mitte seiner
  Beschriftung über den sichtbaren Stunden. Die Beschriftung bleibt innerhalb der Zeichenfläche.
  Platziert wird vom jüngsten Tag rückwärts; überdeckt eine Beschriftung die zuletzt gesetzte,
  entfällt sie. So bleibt „heute“ immer lesbar, ein angeschnittener erster Tag fällt weg.
- Die Breite schätzt `labelWidth` grob über die Zeichenzahl (11 px, Tabellenziffern). Das reicht,
  weil nur Überdeckung vermieden werden muss.
- Die Diagramme wachsen um die Tageszeile nach oben; die Zeichenfläche behält ihre Höhe
  (`<g transform>` statt neuer Koordinaten).

## Datenquellen

- Kosten: `dayCosts(rows)` fasst die Tageszeilen des Kostenberichts zusammen. Sie decken auch
  Kosten ab, die in `hours` fehlen, und markieren Untergrenzen wie die Aufschlüsselung.
- Tokens/s: der Tagesmedian je Modell ist exakt (Backend, `DayPerf`). Ein Median über Stunden
  wäre nur eine Näherung.

## Übersicht

- `TrendCharts.svelte` lädt `costs(7)` und `performance(7)` selbst. Die Übersicht erhöht bei
  jedem Neuladen einen Zähler (`refresh`), damit beide Diagramme mit der Seite aktualisieren.
- Namen der Accounts kommen aus den bereits geladenen Abos der Übersicht.
- Ohne Account-Verwaltung erscheinen die Accounts mit dem Ersatznamen aus #121.

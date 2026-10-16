# Design

## Ein Entwurf, zwei Wege

Der Filter-Bereich hält genau einen Zustand: den YAML-Entwurf als Text. Der Editor bearbeitet
ihn direkt. Das Formular (Schalter, eigene Regeln, Allowlist, Wortlisten-Optionen) ändert ihn
über das Dokumentmodell der Bibliothek `yaml` (`parseDocument`, `setIn`, `addIn`,
`deleteIn`), sodass Kommentare und der Abschnitt `compress:` erhalten bleiben. Dry-Run,
Live-Test und Speichern senden immer diesen Text an dieselben Endpunkte wie ein YAML-Import.
Damit ergeben UI und YAML per Konstruktion dieselbe Konfiguration.

Abnahmetest: `web/src/filters.test.ts` baut eine Konfiguration nur über das Formular und
vergleicht den gesendeten Text mit `web/src/testdata/form-config.yaml`. Der Go-Test
`internal/redactcfg/equivalence_test.go` importiert diese Datei und das handgeschriebene
YAML mit gleichem Inhalt und verlangt bytegleiche Exporte.

Ist der Entwurf syntaktisch kaputt, sperrt das Formular und verweist auf den Editor.

## Live-Validierung

`YamlEditor.svelte` kapselt CodeMirror 6 mit `@codemirror/lang-yaml` und
`codemirror-json-schema/yaml` (ADR-0003). Das Schema kommt zur Laufzeit von
`GET /api/config/schema`, also genau das eingebettete Schema des Servers. Die semantische
Prüfung (Regex kompiliert, Beispiele treffen) kann nur der Server: Der Dry-Run läuft
auf Abruf („Prüfen“ oder Mod+Enter im Editor), nicht automatisch nach jeder Änderung: Jeder
Lauf geht über alle Fixtures, und die Schema-Diagnosen im Editor decken Tippfehler schon live
ab. Seine `issues` (Pfad, Zeile, Spalte) erscheinen als Liste unter dem Editor. Speichern ist erst nach einem grünen Dry-Run des aktuellen Texts
möglich; die Vorschau zeigt pro Fixture die Treffer aktiv gegenüber Entwurf.

## Wortliste ohne Klartext im Browser

Die Detailansicht einer Version liefert die Wortliste nur maskiert. Ein Entwurf aus dieser
Fassung darf die maskierten Einträge nie speichern. Lösung im Backend:

- `DryRunRequest`, `ImportBody` und `TestBody` bekommen `words_from` (Versions-ID, 0 = aus).
  Ist es gesetzt, MUST `words.list` im Entwurf leer sein; die Liste wird aus der
  gespeicherten Version übernommen (Fehler sonst als Validierungs-Issue an `words.list`).
- Jeder Eintrag mit dem Maskierungsfüller `••••••••` wird abgelehnt (`words.list[i]`), ohne
  den Eintrag zu zitieren.

Das Frontend startet einen Entwurf aus der aktiven Version mit `words.list: []` und
`words_from = aktive Version`, solange der Nutzer die Wortliste nicht ersetzt. Die Liste
erscheint maskiert aus der Versionsansicht. Neu eingegebene Wörter stehen bis zum Speichern im
Entwurf (vom Nutzer selbst getippt); gespeicherte Wörter erscheinen nur maskiert.

Export mit Wortliste (`words=plain`) startet erst nach einem Bestätigungsdialog mit dem Text
der Export-Warnung; der Download läuft per `fetch` mit Bearer-Token und `Blob`, nie über eine
URL mit Token.

## Platzhalter in Treffern

Die Übersicht soll den Platzhalter zeigen. `store.Match` hatte nur Offset, Länge, Typ, Regel.
`Placeholder` wird additiv ergänzt (JSON-Spalte, keine Migration; alte Zeilen haben `""`).
`redact.Number` kennt ihn ohnehin. Der Store akzeptiert nur leer oder
`<redacted:[a-z][a-z0-9_]{0,31}_[1-9][0-9]{0,5}>`; damit kann über dieses Feld kein Klartext
gespeichert werden. Der Feld-Allowlist-Test im Store wird bewusst erweitert.

## Letzte Redactions

`store.RecentRedactions(limit)` wählt zuerst die neuesten `limit` Stufenzeilen mit nicht
leerem `matches` (jede hat mindestens einen Treffer), entfaltet sie mit `json_each` und
begrenzt auf `limit` Treffer. Reihenfolge: Anfrage neueste zuerst (`started_at DESC, id DESC`),
innerhalb der Anfrage Stufe und Dokumentreihenfolge. Der Endpunkt
`GET /api/redactions?limit=1..50` (Standard 10) gibt Zeit, Trace-ID, Client-ID und -Name,
Stufe, Typ, Regel und Platzhalter aus.

## Trace-Detail

`GET /api/traces/{id}` gibt `TraceDetail{trace, stages}` zurück. `trace` ist die bekannte
`TraceSummary` (Client-Name über `ListClients`), `stages` die gespeicherten Messwerte in
Pipeline-Reihenfolge mit Diff und Treffern. Unbekannte ID: 404.

Die Timeline in Live zeigt pro Stufe Dauer, Tokens vorher → nachher, gekürzte Hashes
(voller Wert im `title`), Konfigurationsversion, Treffer nach Typ und den Diff. Die Zeile
„Rückweg“ zeigt Antwortstatus, Gesamtdauer, Ausgabe-Tokens und die Zahl der eindeutigen
Platzhalter, die zurückübersetzt werden können. Läuft die Anfrage noch, lädt das Detail beim
Abschluss-Event neu.

## Alternativen

- **Validierung nur per Server:** verworfen, ADR-0008 verlangt Live-Validierung im Editor
  gegen das ausgelieferte Schema. Der Server-Dry-Run ergänzt nur die Semantik.
- **Wortliste per `export?words=plain` in den Browser holen:** verworfen; der Klartext stünde
  im Editor und im Speicher der Seite.
- **Platzhalter aus dem Diff herauslesen:** verworfen; der Diff ist gekürzt und mehrdeutig.

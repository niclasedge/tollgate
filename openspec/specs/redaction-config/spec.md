# redaction-config Specification

## Purpose
Die Redactor-Konfiguration ist versioniert in der DB, über Frontend und YAML gleichwertig pflegbar, vor der Aktivierung per Schema und Dry-Run geprüft und ohne Neustart aktivierbar; die Wortliste bleibt verschlüsselt.

## Requirements

### Requirement: Schema und Validierung

tollgate SHALL die Laufzeitkonfiguration gegen `config/schema/tollgate-config.v1.schema.json` (JSON Schema 2020-12, `additionalProperties: false` auf allen Ebenen) prüfen, das per `embed` im Binary liegt und unter `GET /api/config/schema` ausgeliefert wird. Danach SHALL die semantische Prüfung aus `redact.Compile` laufen. Jeder Fehler MUST Pfad, Zeile und Spalte nennen und MUST NOT Einträge der Wortliste zitieren. Ungültiges YAML, mehrere Dokumente und doppelte Schlüssel MUST mit verständlicher Meldung abgelehnt werden.

#### Scenario: Tippfehler im Schlüssel
- **WHEN** das YAML `builtin.emial: true` enthält
- **THEN** wird es mit Pfad `builtin`, Zeile und Spalte sowie dem Namen `emial` abgelehnt.

#### Scenario: Ungültiger Platzhalter
- **WHEN** `custom[0].placeholder` den Wert `Kunde-ID` hat
- **THEN** nennt die Meldung `custom[0].placeholder`, Zeile, Spalte und die Grammatik.

#### Scenario: Kaputtes YAML
- **WHEN** das YAML syntaktisch ungültig ist
- **THEN** nennt die Meldung die Zeile des Fehlers.

### Requirement: Import, Export und Roundtrip

Der Export SHALL deterministisch aus den Go-Structs entstehen (feste Feldreihenfolge, alle Schalter ausgeschrieben). Import und anschließender Export MUST dieselbe Konfiguration ergeben, und ein zweiter Export MUST bytegleich sein. Ohne ausdrückliche Option MUST der Export die Wortliste weglassen; mit Option MUST die erste Zeile `# WARNUNG: enthält die vertrauliche Wortliste – nicht committen` lauten.

#### Scenario: Roundtrip
- **WHEN** das Beispiel aus AGENTS.md importiert und mit Wortliste exportiert wird
- **THEN** ergibt `Parse` des Exports dasselbe Dokument, und `Export` darauf ist bytegleich.

#### Scenario: Export ohne Wortliste
- **WHEN** die API ohne `words=plain` exportiert
- **THEN** steht `list: []` im YAML und kein Eintrag der Wortliste in der Antwort.

### Requirement: Dry-Run auf Fixtures

Vor dem Import SHALL ein Dry-Run alle Fixtures unter `testdata/fixtures/` mit der aktiven und der neuen Konfiguration redigieren und pro Fixture die Treffer beider Seiten als Pfad, Offset, Länge, Typ und Regel liefern, ohne gespeicherte Werte oder Klartext.

#### Scenario: Neue Wortliste im Dry-Run
- **WHEN** eine Konfiguration mit `Projekt Falke` in der Wortliste gegen die Fixtures geprüft wird
- **THEN** zeigt die synthetische Fixture einen Treffer `word` nur auf der Kandidatenseite.

### Requirement: Versionen, Diff und Rollback

Jeder Import SHALL eine neue, unveränderliche Version mit Zeitstempel, Autor, Kommentar und Diff zur Vorgängerversion anlegen. Der Diff MUST die Wortliste maskiert zeigen. Ein Rollback MUST eine neue Version mit dem Inhalt der Zielversion anlegen und aktivieren. Beim ersten Start ohne aktive Version MUST die Standardkonfiguration als Version 1 angelegt und aktiviert werden.

#### Scenario: Rollback
- **WHEN** Version 2 aktiv ist und auf Version 1 zurückgerollt wird
- **THEN** entsteht Version 3 mit dem Inhalt von Version 1, Version 3 ist aktiv, und Version 2 bleibt unverändert.

#### Scenario: Erster Start
- **WHEN** der Manager auf einer leeren Datenbank öffnet
- **THEN** ist Version 1 aktiv und entspricht `redact.DefaultOptions()`.

### Requirement: Hot Reload und Sitzungs-Pinning

Das Aktivieren SHALL die kompilierte Konfiguration atomar tauschen, ohne Neustart. Parallele Leser MUST immer eine vollständige Version sehen. Eine angefragte Version MUST unabhängig von der aktiven Version geliefert werden, damit gepinnte Sitzungen ihre Regeln behalten.

#### Scenario: Paralleles Lesen beim Aktivieren
- **WHEN** viele Goroutinen redigieren, während wiederholt zwischen zwei Versionen umgeschaltet wird
- **THEN** liefert jeder Aufruf das Ergebnis genau einer der beiden Versionen, und `go test -race` meldet nichts.

#### Scenario: Gepinnte Sitzung
- **WHEN** eine Sitzung auf Version 1 gepinnt ist und Version 2 mit neuer Wortliste aktiviert wird
- **THEN** redigiert `Redactor(ctx, 1)` weiter ohne die neue Wortliste, `Redactor(ctx, 2)` mit.

### Requirement: Verschlüsselte Wortliste

Die Wortliste SHALL nur verschlüsselt über einen `Sealer` mit der Versions-ID als AAD gespeichert werden. Ohne Sealer MUST ein Import mit Wortliste abgelehnt werden und eine gespeicherte Version mit Wortliste nicht geladen werden. Die Wortliste MUST NOT im Klartext in der Datenbank, in Logs oder in API-Antworten außer dem ausdrücklichen Export stehen; die Anzeige ist maskiert.

#### Scenario: Ohne Sealer
- **WHEN** ohne Sealer eine Konfiguration mit Wortliste importiert wird
- **THEN** wird sie abgelehnt und keine Version angelegt.

#### Scenario: Leck-Test
- **WHEN** eine Konfiguration mit Wortliste importiert, aktiviert, gelistet, exportiert (ohne Option), im Dry-Run geprüft und mit fehlerhaftem Eintrag erneut importiert wird
- **THEN** steht kein Eintrag im Klartext in der SQLite-Datei samt WAL, in den Logs oder in den API-Antworten.

### Requirement: Admin-API

tollgate SHALL unter `/api/config/` hinter dem Admin-Token Schema, Dry-Run, Import, Versionsliste, Versionsdetail, Aktivieren, Rollback, Export und Live-Test anbieten. Validierungsfehler MUST mit Status 422 und einer Liste aus Pfad, Zeile, Spalte und Meldung antworten.

#### Scenario: Live-Test
- **WHEN** `POST /api/config/test` mit einem Text voller Treffer aufgerufen wird
- **THEN** enthält die Antwort Start, Ende, Typ und Regel jedes Treffers und den redigierten Text.

### Requirement: Wortliste aus einer Version übernehmen

`POST /api/config/dry-run`, `POST /api/config/import` und `POST /api/config/test` SHALL das Feld `words_from` (Versions-ID, 0 = aus) annehmen. Ist es gesetzt, MUST die Wortliste der gespeicherten Version gelten, und `words.list` im Entwurf MUST leer sein; andernfalls wird mit 422 und einem Issue an `words.list` abgelehnt. Eine unbekannte Version MUST mit 404 abgelehnt werden. Ein Wortlisten-Eintrag, der den Maskierungsfüller `••••••••` enthält, MUST als Validierungsfehler am Pfad des Eintrags abgelehnt werden, ohne den Eintrag zu zitieren.

#### Scenario: Wortliste übernehmen
- **WHEN** Version 2 die Wortliste `Projekt Falke` hat und ein Entwurf mit `words.list: []` und `words_from: 2` importiert wird
- **THEN** hat die neue Version dieselbe Wortliste, und die Antworten enthalten den Eintrag nicht im Klartext.

#### Scenario: Maskierte Fassung importiert
- **WHEN** das YAML aus `GET /api/config/versions/2` unverändert importiert wird
- **THEN** antwortet die API mit 422 und einem Issue an `words.list[0]`.

#### Scenario: Doppelte Wortliste
- **WHEN** ein Entwurf eine Wortliste enthält und `words_from` gesetzt ist
- **THEN** antwortet die API mit 422 und einem Issue an `words.list`.

#### Scenario: UI und YAML gleichwertig
- **WHEN** die über das Formular erzeugte Konfiguration und dasselbe handgeschriebene YAML importiert werden
- **THEN** sind beide Exporte bytegleich.

### Requirement: Wortliste per Operationen ändern

`POST /api/config/dry-run`, `POST /api/config/import` und `POST /api/config/test` SHALL zusätzlich die Felder `words_remove` (Indizes der Basisliste) und `words_add` (neue Wörter) annehmen. Basisliste MUST die Wortliste aus `words_from` sein, ohne `words_from` die Liste im Entwurf. Das Ergebnis MUST deterministisch sein: Basisliste ohne die entfernten Indizes in ihrer Reihenfolge, danach die neuen Wörter getrimmt in Eingabereihenfolge; leere neue Einträge und neue Einträge, die gemäß `case_sensitive` schon im Ergebnis stehen, MUST verworfen werden. Ein Index außerhalb der Basisliste oder ein doppelter Index MUST mit 400 und einer Meldung abgelehnt werden, die nur Indizes nennt. Ein neues Wort, das die Schemagrenzen verletzt oder den Maskierungsfüller enthält, MUST mit 422 und einem Issue an `words_add[i]` abgelehnt werden, ohne das Wort zu zitieren. Antworten, Logs und der Diff der neuen Version MUST NOT ein Wort der Liste im Klartext enthalten.

#### Scenario: Wort hinzufügen
- **WHEN** Version 2 die Wortliste `Projekt Falke`, `Muster GmbH` hat und ein Entwurf mit `words_from: 2` und `words_add: ["Neu AG"]` importiert wird
- **THEN** hat die neue Version die Wortliste `Projekt Falke`, `Muster GmbH`, `Neu AG`.

#### Scenario: Eintrag entfernen
- **WHEN** derselbe Entwurf stattdessen `words_remove: [0]` sendet
- **THEN** hat die neue Version die Wortliste `Muster GmbH`.

#### Scenario: Dubletten ohne Groß-/Kleinschreibung
- **WHEN** bei `case_sensitive: false` `words_add: ["muster gmbh", " ", "Neu AG", "neu ag"]` gesendet wird
- **THEN** kommt nur `Neu AG` hinzu.

#### Scenario: Dubletten mit Groß-/Kleinschreibung
- **WHEN** bei `case_sensitive: true` `words_add: ["muster gmbh", "Muster GmbH"]` gesendet wird
- **THEN** kommt nur `muster gmbh` hinzu.

#### Scenario: Ändern durch Entfernen und Hinzufügen
- **WHEN** `words_remove: [1]` und `words_add: ["Muster GmbH"]` gesendet werden
- **THEN** steht `Muster GmbH` am Ende der Liste.

#### Scenario: Ungültiger Index
- **WHEN** `words_remove: [5]` bei einer Basisliste mit zwei Einträgen gesendet wird
- **THEN** antwortet die API mit 400 und einer Meldung, die Index und Länge nennt, aber kein Wort.

#### Scenario: Kein Klartext
- **WHEN** Dry-Run, Test und Import mit Operationen laufen
- **THEN** enthalten weder Antworten noch Logs noch die Datenbankdateien ein Wort der Liste im Klartext.

### Requirement: Namenslisten im Schema

Der Block `words` SHALL zusätzlich `customers`, `first_names` und `last_names` kennen (je höchstens 5000 Einträge mit 1 bis 200 Zeichen, fehlend = leer) und `case_sensitive` und `whole_word` mit `list` teilen. `words.list` MUST unverändert bleiben; Konfigurationen und gespeicherte Versionen ohne die neuen Schlüssel MUST gültig bleiben.

#### Scenario: Altes YAML
- **WHEN** ein YAML ohne `customers`, `first_names` und `last_names` importiert wird
- **THEN** ist es gültig und die drei Listen sind leer.

### Requirement: Namenslisten in Import und Export

Der Export SHALL alle vier Listen ausschreiben: ohne Wortlisten als `[]`, mit `words=plain` im Klartext hinter der Warnung, in Versionsansichten maskiert. Import und Export MUST dieselbe Konfiguration ergeben. Maskierte Einträge MUST beim Import mit einem Issue am Pfad der Liste abgelehnt werden, ohne den Eintrag zu zitieren.

#### Scenario: Round-Trip
- **WHEN** ein YAML mit allen vier Listen importiert und mit `words=plain` exportiert wird
- **THEN** ergibt der Re-Import dieselbe Konfiguration.

#### Scenario: Export ohne Wortlisten
- **WHEN** der Standardexport einer Version mit allen Listen angefordert wird
- **THEN** stehen `list`, `customers`, `first_names` und `last_names` als `[]` im Dokument, kein Eintrag im Klartext.

### Requirement: Versiegelung aller Listen

Die vier Listen SHALL gemeinsam als ein Objekt versiegelt gespeichert werden (AAD wie bisher); in `config_versions.content` MUST kein Eintrag stehen. Das alte Format (versiegeltes JSON-Array, nur `words.list`) MUST weiter gelesen werden. Antworten, Diffs, Logs, Fehlermeldungen und Datenbankdateien MUST NOT einen Eintrag im Klartext enthalten.

#### Scenario: Altes Format
- **WHEN** eine Version mit einem als JSON-Array versiegelten Blob geladen wird
- **THEN** ist `words.list` der Inhalt des Arrays und die drei Listen sind leer.

#### Scenario: Kein Klartext
- **WHEN** Versionen mit Einträgen in allen Listen gespeichert, gelesen, exportiert (ohne Klartext), zurückgerollt und getestet werden
- **THEN** enthalten weder Datenbankdateien noch Logs noch Antworten einen Eintrag.

### Requirement: Listen per `lists_set` ersetzen

`POST /api/config/dry-run`, `/import` und `/test` SHALL ein Feld `lists_set` mit `customers`, `first_names` und `last_names` annehmen. Jede vorhandene Liste MUST die Liste von `words_from` bzw. des Entwurfs vollständig ersetzen (ein leeres Array leert sie), eine fehlende Liste MUST unverändert bleiben. Das Ergebnis MUST deterministisch sein: Einträge getrimmt, leere Einträge und Dubletten gemäß `case_sensitive` verworfen, Reihenfolge der Eingabe.

#### Scenario: Ersetzen
- **WHEN** Version 2 die Kunden `A` und `B` hat und ein Entwurf mit `words_from: 2` und `lists_set: {customers: ["C", " ", "c"]}` importiert wird
- **THEN** hat die neue Version den Kunden `C`, die Vornamen und Nachnamen und `words.list` bleiben wie in Version 2.

#### Scenario: Leeres Array leert die Liste
- **WHEN** derselbe Entwurf `lists_set: {first_names: []}` sendet
- **THEN** hat die neue Version keine Vornamen, Kunden und Nachnamen bleiben wie in Version 2.

### Requirement: Grenzen und Übernahme bei `lists_set`

Ein Eintrag in `lists_set`, der die Schemagrenzen verletzt oder den Maskierungsfüller enthält, MUST mit 422 und einem Issue an `lists_set.<liste>[i]` abgelehnt werden, ohne den Eintrag zu zitieren. `words_from` MUST alle vier Listen der Version übernehmen; steht im Entwurf zugleich eine der vier Listen, MUST die Anfrage mit 422 abgelehnt werden. `words_add` und `words_remove` MUST weiter auf `words.list` wirken.

#### Scenario: Übernahme ohne `lists_set`
- **WHEN** ein Entwurf nur `words_from: 2` sendet
- **THEN** hat die neue Version alle vier Listen von Version 2.

#### Scenario: Grenzverletzung
- **WHEN** ein Eintrag länger als 200 Zeichen ist
- **THEN** antwortet die API mit 422 und einem Issue an `lists_set.customers[0]`, ohne den Eintrag zu zitieren.

### Requirement: Klartext der Namenslisten nur auf Anfrage

`GET /api/config/lists` SHALL hinter dem Admin-Token die Listen Kunden, Vornamen und Nachnamen einer Version im Klartext liefern (`version` fehlt = aktive Version) mit `Cache-Control: no-store`; die allgemeine Wortliste MUST NOT enthalten sein. Bei unbekannter Version MUST er mit 404 antworten, ohne Schlüssel mit 422. Geloggt MUST nur die Versionsnummer werden, nie ein Eintrag.

#### Scenario: Klartext auf Anfrage
- **WHEN** der Admin `GET /api/config/lists?version=2` aufruft
- **THEN** enthält die Antwort die drei Listen im Klartext und `Cache-Control: no-store`.

#### Scenario: Kein Klartext in Log und Datenbank
- **WHEN** der Endpunkt aufgerufen wurde
- **THEN** enthalten Logs und Datenbankdateien weiterhin keinen Eintrag im Klartext.

### Requirement: Neue Listen gelten erst für neue Sitzungen

Eine Konfigurationsversion mit Namenslisten SHALL nur für Sitzungen gelten, die nach der Aktivierung beginnen; Sitzungen mit gepinnter älterer Version MUST ihre Regeln und damit ihre Präfixe behalten.

#### Scenario: Gepinnte Sitzung
- **WHEN** eine Version mit Kunde `Muster GmbH` aktiviert wird, während eine Sitzung auf die Vorversion gepinnt ist
- **THEN** ersetzt die gepinnte Sitzung `Muster GmbH` weiter nicht, eine neue Sitzung ersetzt es durch `<redacted:kunde_1>`.

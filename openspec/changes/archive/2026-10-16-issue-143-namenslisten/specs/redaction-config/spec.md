## ADDED Requirements

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

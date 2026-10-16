## ADDED Requirements

### Requirement: Filter-Bereich

Der Bereich Filter SHALL die aktive Redactor-Konfiguration als Liste zeigen (eingebaute Filter als Schalter, eigene Regeln, Wortliste nur maskiert, Allowlist) und einen YAML-Entwurf bearbeiten lassen, entweder über das Formular oder über einen CodeMirror-Editor. Beide Wege MUST denselben Entwurf ändern und über denselben Import-Endpunkt speichern. Der Editor MUST live gegen das von `GET /api/config/schema` ausgelieferte Schema validieren. Vor dem Speichern MUST ein Dry-Run des aktuellen Entwurfs mit Vorschau pro Fixture (aktiv gegenüber Entwurf) stattgefunden haben; Validierungsfehler MUST mit Pfad, Zeile und Spalte erscheinen. Der Bereich SHALL einen Live-Test (Text einfügen, Treffer und redigierten Text sehen), die Versionshistorie mit Diff, Aktivieren und Rollback sowie Import aus Datei und Export bieten. Ein Export mit Wortliste MUST vorher die Warnung „enthält die vertrauliche Wortliste – nicht committen“ bestätigen lassen. Solange der Nutzer die Wortliste nicht ersetzt, MUST der Entwurf sie per `words_from` übernehmen, statt maskierte Einträge zu senden.

#### Scenario: Formular und YAML gleichwertig
- **WHEN** der Nutzer im Formular einen Schalter ändert, eine eigene Regel und einen Allowlist-Eintrag hinzufügt und speichert
- **THEN** sendet das Frontend genau den YAML-Text aus `web/src/testdata/form-config.yaml` an den Import-Endpunkt, und der Editor zeigt diesen Text.

#### Scenario: Speichern erst nach Dry-Run
- **WHEN** der Entwurf geändert wurde und der Dry-Run Validierungsfehler meldet
- **THEN** zeigt der Bereich die Fehler mit Zeile und Spalte, und Speichern ist gesperrt.

#### Scenario: Wortliste bleibt maskiert
- **WHEN** die aktive Version eine Wortliste hat und der Bereich geladen wird
- **THEN** erscheinen nur maskierte Einträge, und Dry-Run und Import senden `words_from` mit leerer Liste.

#### Scenario: Export mit Wortliste
- **WHEN** der Nutzer den Export mit Wortliste wählt
- **THEN** erscheint die Warnung, und erst nach Bestätigung wird `export?words=plain` abgerufen.

#### Scenario: Rollback
- **WHEN** der Nutzer in der Versionshistorie bei einer älteren Version „Zurückrollen“ wählt
- **THEN** ruft das Frontend `POST /api/config/versions/{id}/rollback` auf und zeigt die neue aktive Version.

### Requirement: Stufen-Timeline in Live

Die aufgeklappte Detailansicht einer Anfrage in Live SHALL ihre Stufen aus `GET /api/traces/{id}` als Timeline zeigen: pro Stufe Dauer, Tokens vorher und nachher, Hashes, Konfigurationsversion, Treffer und den gespeicherten Diff. Der Rückweg MUST als eigene Zeile erscheinen.

#### Scenario: Timeline einer redigierten Anfrage
- **WHEN** der Nutzer eine Anfrage mit Stufen `intake` und `redact` aufklappt
- **THEN** zeigt die Timeline beide Stufen mit Tokens vorher → nachher, den Diff der Redact-Stufe und eine Zeile „Rückweg“.

### Requirement: Letzte Redactions auf der Übersicht

Die Übersicht SHALL die letzten 10 Redactions aus `GET /api/redactions?limit=10` zeigen: Zeit, Client, Typ, Platzhalter und Regel, mit Link ins Live-Detail der Anfrage. Ohne Treffer MUST „Noch keine Redactions“ erscheinen.

#### Scenario: Panel mit Treffern
- **WHEN** die API zwei Treffer liefert
- **THEN** zeigt das Panel beide mit Platzhalter und Typ und verlinkt `#/live/<trace-id>`.

## ADDED Requirements

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

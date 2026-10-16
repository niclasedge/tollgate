## ADDED Requirements

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

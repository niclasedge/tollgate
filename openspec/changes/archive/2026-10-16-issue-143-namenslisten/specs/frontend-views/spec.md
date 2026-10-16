## ADDED Requirements

### Requirement: Namenslisten maskiert im Filter-Bereich

Der Filter-Bereich SHALL neben der Wortliste die Abschnitte Kunden, Vornamen und Nachnamen zeigen, je mit Anzahl und maskierten Einträgen. Der Klartext MUST erst auf den Klick „Bearbeiten“ von `GET /api/config/lists` geladen werden (bei übernommener Version; sonst aus dem Entwurf) und nur im Komponentenzustand liegen.

#### Scenario: Maskiert ohne Klick
- **WHEN** die aktive Version Kunden hat und der Filter-Bereich geladen wird
- **THEN** zeigt der Abschnitt Kunden Anzahl und maskierte Einträge und es gibt keinen Aufruf von `/api/config/lists`.

### Requirement: Namenslisten im Textfeld bearbeiten

„Bearbeiten“ SHALL das Textfeld der gewählten Liste mit einem Eintrag je Zeile öffnen; „Übernehmen“ MUST Zeilen trimmen und leere Zeilen und Dubletten (gemäß `case_sensitive`) verwerfen. Bei übernommener Version MUST das Ergebnis als `lists_set` neben `words_from` gesendet werden, der YAML-Entwurf MUST die Listen dann nicht enthalten.

#### Scenario: Bearbeiten und Speichern
- **WHEN** der Nutzer „Bearbeiten“ wählt, im Textfeld eine Zeile ergänzt, übernimmt, prüft und speichert
- **THEN** wurde `/api/config/lists` genau einmal aufgerufen und Dry-Run und Import senden `words_from` und `lists_set` mit der vollständigen bearbeiteten Liste (nur diese).

#### Scenario: Leere Zeilen und Dubletten
- **WHEN** das Textfeld `A`, eine leere Zeile, `a` und `B ` enthält und `case_sensitive` aus ist
- **THEN** enthält `lists_set` genau `A` und `B`.

### Requirement: Namenslisten-Änderung braucht Prüfen und ist bedienbar

Jede Änderung an den Namenslisten MUST den Dry-Run ungültig machen; gespeichert wird erst nach „Prüfen“. Alle Bedienelemente MUST per Tastatur erreichbar sein; Dark Mode und schmale Breite MUST wie im Rest des Bereichs funktionieren.

#### Scenario: Änderung sperrt Speichern
- **WHEN** nach erfolgreichem „Prüfen“ eine Liste übernommen wird
- **THEN** ist „Speichern und aktivieren“ gesperrt.

### Requirement: Wortliste ersetzen erhält die Namenslisten

„Wortliste ersetzen“ und „Wortliste entfernen“ MUST die Kunden-, Vornamen- und Nachnamenlisten einer übernommenen Version erhalten.

#### Scenario: Wortliste ersetzen
- **WHEN** die übernommene Version Kunden hat und der Nutzer die allgemeine Wortliste ersetzt
- **THEN** bleibt `words_from` gesetzt und die Kunden gehen nicht verloren.

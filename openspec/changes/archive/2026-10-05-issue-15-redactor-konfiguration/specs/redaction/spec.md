## ADDED Requirements

### Requirement: Eigene Regex-Filter

`redact.Compile` SHALL eigene Regeln aus `{name, pattern, placeholder, examples}` mit Priorität 100 über den eingebauten Regeln einbauen. Eine Konfiguration MUST abgelehnt werden, wenn ein Muster nicht kompiliert, den leeren Text trifft, ein Beispiel nicht trifft, ein Name doppelt ist oder der Platzhalter-Typ nicht der Grammatik `[a-z][a-z0-9_]{0,31}` aus ADR-0006 entspricht oder einem eingebauten Typ bzw. `word` gleicht.

#### Scenario: Eigene Kennung
- **WHEN** die Regel `kundennummer` mit Muster `KD-\d{6}` und Platzhalter `customer_id` aktiv ist und der Text `Ticket KD-123456` lautet
- **THEN** lautet die Ausgabe `Ticket <redacted:customer_id>`, Regel `custom:kundennummer`.

#### Scenario: Beispiel trifft nicht
- **WHEN** die Regel als Beispiel `KD-12` angibt
- **THEN** schlägt `Compile` mit dem Pfad `custom[0].examples[0]` fehl.

### Requirement: Wortliste

`redact.Compile` SHALL jeden Eintrag der Wortliste mit Priorität 110 und Typ `word` erkennen. `case_sensitive: false` MUST Groß- und Kleinschreibung Unicode-bewusst ignorieren. `whole_word: true` MUST Treffer verwerfen, deren Nachbarzeichen Buchstabe, Ziffer oder Kombinationszeichen sind, sofern der Rand des Eintrags selbst ein solches Zeichen ist. Mehrwort-Einträge MUST mit beliebigem Whitespace einschließlich Zeilenumbruch treffen. Fehlermeldungen MUST NOT Einträge der Wortliste zitieren. Die Reihenfolge MUST unabhängig von Map-Iteration sein.

#### Scenario: Ganze Wörter mit Umlaut
- **WHEN** die Wortliste `Müller` mit `whole_word: true` enthält und der Text `Müllers Akte, Herr Müller.` lautet
- **THEN** wird nur das zweite Vorkommen ersetzt; mit `whole_word: false` beide.

#### Scenario: Groß- und Kleinschreibung
- **WHEN** `case_sensitive: false` gilt und der Text `MUSTER GMBH` enthält
- **THEN** wird `Muster GmbH` erkannt; mit `case_sensitive: true` nicht.

#### Scenario: Mehrwort über Zeilenumbruch
- **WHEN** die Wortliste `Projekt Falke` enthält und der Text `Projekt\nFalke` lautet
- **THEN** wird die ganze Spanne ersetzt.

### Requirement: Allowlist

Ein Wert der Allowlist SHALL nie ersetzt werden, gleich welcher Detektor ihn findet. Der Vergleich MUST exakt auf dem Wert erfolgen, der ersetzt würde. Die eingebauten E-Mail-Ausnahmen MUST zusätzlich aktiv bleiben.

#### Scenario: Allowlist hat Vorrang
- **WHEN** die Allowlist `team@muster-gmbh.invalid` enthält und der Text diese und `jane@muster-gmbh.invalid` enthält
- **THEN** bleibt die erste Adresse stehen und die zweite wird ersetzt.

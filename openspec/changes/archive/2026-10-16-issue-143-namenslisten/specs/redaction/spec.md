## ADDED Requirements

### Requirement: Kunden-, Vornamen- und Nachnamenlisten

`redact.Compile` SHALL neben der Wortliste drei weitere Listen annehmen: Kunden mit dem Platzhalter-Typ `kunde`, Vornamen mit `vorname` und Nachnamen mit `nachname`. Sie MUST dieselbe Treffersemantik wie die Wortliste haben (`case_sensitive`, `whole_word`, Unicode-Wortgrenzen, mehrteilige Einträge, längster Treffer zuerst, Allowlist, Vorrang vor eigenen Regeln und eingebauten Filtern).

#### Scenario: Kunde, Vorname und Nachname
- **WHEN** Kunde `Muster GmbH`, Vorname `Anna` und Nachname `Müller` konfiguriert sind und der Text `Anna Müller von Muster GmbH` redigiert wird
- **THEN** lautet das Ergebnis `<redacted:vorname_1> <redacted:nachname_1> von <redacted:kunde_1>`.

#### Scenario: Längster Eintrag gewinnt
- **WHEN** Kunde `Anna Müller GmbH` und Vorname `Anna` konfiguriert sind und `Anna Müller GmbH` im Text steht
- **THEN** entsteht genau ein Treffer vom Typ `kunde`.

### Requirement: Namenslisten sind deterministisch und rückübersetzbar

Die Ersetzung durch die drei Listen MUST deterministisch sein: gleiche Eingabe und Konfiguration, Byte für Byte gleiche Ausgabe, unabhängig von der Reihenfolge der Einträge. Die Platzhalter MUST je Typ nummeriert werden. Die Rückübersetzung MUST sie wie alle Platzhalter behandeln, auch über Chunk-Grenzen im Stream und in `input_json_delta`.

#### Scenario: Reihenfolge der Einträge
- **WHEN** dieselben Einträge in zwei Reihenfolgen konfiguriert werden
- **THEN** ist die Ausgabe gleich.

#### Scenario: Rückweg im Stream
- **WHEN** ein Antwort-Stream `<redacted:kunde_1>` auf zwei Chunks verteilt zurückliefert
- **THEN** erscheint der Kundenname im Klartext, ohne Fragment des Platzhalters.

### Requirement: Namenslisten verraten nichts in Regelname und Maske

Der Regelname eines Treffers MUST die Liste nennen und nie den Eintrag. Die Maske eines Treffers der drei Typen MUST nur die Länge zeigen (`••• (N)`), nie ein Zeichen des Eintrags.

#### Scenario: Maske ohne erstes Zeichen
- **WHEN** ein Treffer vom Typ `nachname` maskiert wird
- **THEN** zeigt die Maske nur die Länge.

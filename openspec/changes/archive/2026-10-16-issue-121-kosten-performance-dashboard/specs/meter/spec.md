## ADDED Requirements

### Requirement: Cache-Ersparnis

tollgate SHALL die Ersparnis durch Cache-Lesen einer Usage als `cache_read · (Preis Eingabe − Preis Cache-Lesen)` in Nano-USD ganzzahlig berechnen. Cache-Schreiben MUST NOT eingehen. Für ein unbekanntes Modell MUST die Berechnung „unbekannt“ melden statt eines Werts.

#### Scenario: Opus 5.5
- **WHEN** eine Million Cache-Read-Tokens für `claude-opus-5-5` bewertet werden
- **THEN** ergibt sich eine Ersparnis von 3,80 USD.

#### Scenario: Unbekanntes Modell
- **WHEN** ein Modell keinem Präfix entspricht
- **THEN** meldet die Berechnung keine Ersparnis.

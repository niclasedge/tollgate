## MODIFIED Requirements

### Requirement: Kosten pro Modell

tollgate SHALL die Kosten einer Usage aus den Listenpreisen des Modells in Nano-USD berechnen, wobei das längste an einer Grenze endende Modell-Präfix gilt. Für ein unbekanntes Modell MUST die Berechnung „unbekannt“ melden statt eines Werts. Hat ein Modell nach Prompt-Länge gestaffelte Preise, SHALL die niedrigste Stufe gelten; die Kosten sind dann eine Untergrenze.

#### Scenario: Opus 5.5
- **WHEN** je eine Million Eingabe-, Ausgabe-, Cache-Read- und Cache-Write-Tokens für `claude-opus-5-5` bewertet werden
- **THEN** ergibt sich 29,20 USD.

#### Scenario: Haiku 5.5
- **WHEN** je eine Million Eingabe-, Ausgabe-, Cache-Read- und Cache-Write-Tokens für `claude-haiku-5-5` bewertet werden
- **THEN** ergibt sich 0,735 USD (Stufe bis 100K Prompt-Tokens).

#### Scenario: Unbekanntes Modell
- **WHEN** ein Modell keinem Präfix entspricht
- **THEN** meldet die Berechnung keine Kosten.

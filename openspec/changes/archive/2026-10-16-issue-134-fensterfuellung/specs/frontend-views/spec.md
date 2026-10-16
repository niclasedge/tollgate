## ADDED Requirements

### Requirement: Kennzahlen-Kacheln unter Kosten & Performance

Kosten & Performance SHALL die Kennzahlen als Kacheln mit einer großen Zahl und einer kleineren Zeile darunter zeigen: Tokens verarbeitet (darunter Ausgabe und Eingabe ungecacht), Cache-Ersparnis (darunter Cache gelesen), Von tollgate gespart und Schnellstes 5-h-Fenster. Die Fenster-Kachel MUST groß die hochgerechnete Zeit bis 100 % zeigen und darunter Anstieg, beobachtete Dauer, Ø Anfragen/min und den Account. Ohne Wert MUST sie „–“ und einen Hinweis zeigen, dass die Messung erst ab dem Deploy läuft.

#### Scenario: Fenster vorhanden
- **WHEN** `fastest_fill` 100 min bis 100 % bei 2,5 Anfragen/min meldet
- **THEN** zeigt die Kachel groß „1 h 40 min“ und darunter „0 → 100 % in 1 h 40 min · Ø 2,5 Anfragen/min“ mit dem Account-Namen.

#### Scenario: Noch keine Messung
- **WHEN** `fastest_fill` `null` ist
- **THEN** zeigt die Kachel „–“ und den Hinweis.

## ADDED Requirements

### Requirement: Regelauswahl nach Token-Analyse

Die Auswahl der eingebauten Kompressionsregeln SHALL auf der Rangliste in `docs/token-analyse.md` beruhen. Die Rangliste MUST Methode, Datenquelle, Zeitraum, Bytes je Befehlsgruppe und den Anteil der Ausgaben ab `min_bytes` nennen. Sie MUST NOT Projektnamen, Pfade oder Inhalte der ausgewerteten Ausgaben enthalten.

#### Scenario: Dateilesen per Bash
- **WHEN** die Rangliste `sed -n`, `cat`, `head` und `tail` als größten Block ausweist
- **THEN** bekommen diese Befehle keine eingebaute Regel, weil ihre Ausgabe Dateiinhalt ist.

#### Scenario: Suche gerechtfertigt
- **WHEN** `grep`, `rg` und `git grep` zusammen über 10 % der Bash-Bytes ausmachen
- **THEN** wird die optionale Strategie `search` aus ADR-0007 umgesetzt.

## ADDED Requirements

### Requirement: Bereich Kompression

Der Bereich „Kompression“ SHALL die Regeln der aktiven Konfigurationsversion in Prüfreihenfolge mit Strategie, Befehlspräfixen und Grenzen zeigen und eigene Regeln kennzeichnen. Er MUST anzeigen, ob die Compress-Stufe läuft. Ein Playground SHALL Befehl und Ausgabe annehmen und jeden Schritt mit Zeilen- und Bytezahl zeigen; die Schritte MUST inline aufklappen, das Ergebnis ist offen.

#### Scenario: Stufe aus
- **WHEN** die Compress-Stufe abgeschaltet ist
- **THEN** zeigt der Bereich „Aus“ und der Playground funktioniert trotzdem.

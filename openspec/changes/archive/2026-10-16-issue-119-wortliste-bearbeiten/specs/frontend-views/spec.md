## ADDED Requirements

### Requirement: Wortliste ergänzen und einzeln entfernen

Der Abschnitt Wortliste im Filter-Bereich SHALL ein Feld „Wörter hinzufügen“ (ein Eintrag je Zeile oder kommagetrennt) und an jedem maskierten Eintrag „Entfernen“ bieten; „Wortliste ersetzen“ MUST erhalten bleiben. Hinzugefügte Wörter MUST als `words_add`, entfernte Einträge einer übernommenen Liste MUST als Index in `words_remove` neben `words_from` gesendet werden. Die Liste MUST weiterhin nur maskiert erscheinen, auch für neu hinzugefügte Wörter, und der YAML-Entwurf MUST NOT die übernommene Liste oder maskierte Einträge enthalten. Jede Operation MUST den Dry-Run ungültig machen, Speichern bleibt bis zum nächsten erfolgreichen „Prüfen“ gesperrt. Alle Bedienelemente MUST per Tastatur erreichbar sein.

#### Scenario: Wörter hinzufügen
- **WHEN** die aktive Version 2 eine Wortliste hat und der Nutzer `Neu AG, Zweite KG` hinzufügt, prüft und speichert
- **THEN** senden Dry-Run und Import `words_from: 2` und `words_add: ["Neu AG", "Zweite KG"]`, und die Liste zeigt die neuen Einträge nur maskiert.

#### Scenario: Eintrag entfernen
- **WHEN** der Nutzer beim zweiten Eintrag „Entfernen“ wählt, prüft und speichert
- **THEN** senden Dry-Run und Import `words_remove: [1]`, und der Eintrag verschwindet aus der Anzeige.

#### Scenario: Operation macht Dry-Run ungültig
- **WHEN** nach erfolgreichem „Prüfen“ ein Wort hinzugefügt wird
- **THEN** ist „Speichern und aktivieren“ gesperrt.

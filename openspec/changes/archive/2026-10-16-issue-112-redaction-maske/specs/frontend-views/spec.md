## ADDED Requirements
### Requirement: Masken in Timeline und Letzten Redactions
Die Redact-Stufe der Stufen-Timeline SHALL je Platzhalter eine Zeile mit Typ, Platzhalter und Maske zeigen, mehrfache Treffer desselben Platzhalters mit Anzahl. Das Panel „Letzte Redactions“ SHALL die Maske je Treffer zeigen. Fehlt die Maske (ältere Traces), MUST ein Strich `–` erscheinen.

#### Scenario: Timeline mit Masken
- **WHEN** die Redact-Stufe zwei Treffer `<redacted:email_1>` mit Maske `n•••@f•••.de` und einen Treffer `<redacted:api_key_1>` hat
- **THEN** zeigt die Timeline eine Zeile `email` · `<redacted:email_1>` · `n•••@f•••.de` · `2 ×` und eine Zeile für den API-Key mit seiner Maske

#### Scenario: Übersicht mit Maske
- **WHEN** `/api/redactions` einen Treffer mit Maske liefert und einen ohne
- **THEN** zeigt das Panel beim ersten die Maske und beim zweiten `–`

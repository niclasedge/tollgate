## ADDED Requirements
### Requirement: Maske in gespeicherten Treffern
Ein gespeicherter Treffer SHALL zusätzlich seine Maske enthalten, und `RecentRedactions` SHALL sie mitliefern. Der Store MUST nur eine leere Maske oder eine Maske mit mindestens einem `•`, gültigem UTF-8, ohne Steuerzeichen und mit höchstens 64 Bytes annehmen und andere Werte mit `ErrInvalid` ablehnen. Treffer, die vor der Maske gespeichert wurden, liefern eine leere Maske.

#### Scenario: Klartext im Maskenfeld
- **WHEN** ein Treffer mit der Maske `geheim123` gespeichert werden soll
- **THEN** lehnt der Store den Trace mit `ErrInvalid` ab

#### Scenario: Maske in den letzten Treffern
- **WHEN** ein Treffer mit Maske `n•••@f•••.de` gespeichert und `RecentRedactions(10)` gelesen wird
- **THEN** trägt der Treffer genau diese Maske

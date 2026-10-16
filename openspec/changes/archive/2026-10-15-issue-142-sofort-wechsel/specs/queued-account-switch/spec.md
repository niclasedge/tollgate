## ADDED Requirements

### Requirement: Erzwungener Wechsel
Während ein Wechsel vorgemerkt ist, SHALL ein Admin ihn sofort erzwingen können. tollgate SHALL dann alle aufgenommenen Anfragen abbrechen, worauf der vorgemerkte Wechsel ohne weiteren Klick ausgeführt wird. Ohne Vormerkung SHALL keine Anfrage abgebrochen werden.
#### Scenario: Sofort wechseln bei laufendem Stream
- **WHEN** ein Wechsel auf eine laufende Anfrage wartet und „Sofort wechseln“ gewählt wird
- **THEN** wird die Anfrage abgebrochen, und der Wechsel läuft unmittelbar danach
#### Scenario: Abbruch vor Antwortbeginn
- **WHEN** die abgebrochene Anfrage noch keine Antwort-Header gesendet hat
- **THEN** erhält der Client 503 `overloaded_error` mit `Retry-After`, und der Trace hat die Fehlerart `account_switch`
#### Scenario: Keine Vormerkung
- **WHEN** ein Erzwingen ohne vorgemerkten Wechsel angefordert wird
- **THEN** wird es mit 409 abgelehnt, und laufende Anfragen bleiben unberührt

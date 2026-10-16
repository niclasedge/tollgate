# queued-account-switch Specification

## Purpose
Ein Account-Wechsel lässt sich vormerken; tollgate wartet laufende Anfragen ab, hält neue kurz an und tauscht dann den Account, ohne Anfragen abzubrechen.

## Requirements

### Requirement: Vorgemerkter Account-Wechsel
Ein Account-Wechsel SHALL zuerst exklusiv vorgemerkt werden, alle bereits aufgenommenen Requests vollständig abwarten und erst dann Token-Mutation und Beispielaufruf ausführen. Neue Requests SHALL während Wartephase und Wechsel warten und danach den neuen Account verwenden. Die Wartephase SHALL keinen Manager-Mutex halten, den laufende Requests zur ID-Auflösung benötigen.
#### Scenario: Laufender Stream
- **WHEN** ein Wechsel bei laufendem Stream angefordert wird
- **THEN** bleiben alter Token und alte Trace-ID bis Stream-Ende erhalten, und der Wechsel wird anschließend automatisch ausgeführt
#### Scenario: Neue Anfrage
- **WHEN** während eines vorgemerkten Wechsels eine weitere Anfrage eintrifft
- **THEN** beginnt deren Verarbeitung nach Wechsel-Abschluss mit der neuen Account-ID statt einer Busy-Ablehnung
#### Scenario: Abgebrochenes Warten
- **WHEN** der Kontext der Vormerkung vor Stream-Ende abbricht
- **THEN** bleiben Tokens unverändert, und wartende Requests können den bisherigen Account verwenden
#### Scenario: Konkurrierender Wechsel
- **WHEN** eine weitere Vormerkung bei bereits vorgemerktem Wechsel eintrifft
- **THEN** wird sie eindeutig abgewiesen, ohne die vorhandene Vormerkung zu überschreiben

### Requirement: Sichtbarer Fortschritt
Das Frontend SHALL während der Vormerkung die Anzahl laufender Anfragen und anschließend Aktivierung/Test zeigen. Der Status-Read SHALL nicht auf den Manager-Mutex warten.
#### Scenario: Automatische Durchführung
- **WHEN** die Zahl laufender Anfragen auf null sinkt
- **THEN** wechselt der Fortschritt von Warten zu Aktivierung/Test ohne erneuten Klick

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

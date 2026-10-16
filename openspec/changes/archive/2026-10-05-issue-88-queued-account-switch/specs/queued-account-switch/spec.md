## ADDED Requirements
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

## MODIFIED Requirements

### Requirement: Abo-Karten nach Verfügbarkeit

Die Übersicht SHALL die Abo-Karten in dieser Reihenfolge zeigen: das aktive Abo; Abos mit Wochenkontingent und freiem 5-h-Fenster, nach niedrigster Wochenauslastung und bei Gleichstand nach niedrigster 5-h-Auslastung; Abos mit Wochenkontingent und vollem 5-h-Fenster, nach nächstem 5-h-Reset; Abos mit verbrauchtem Wochenkontingent, nach Wochen-Reset; Abos ohne Nutzungsdaten. Ein Wochenkontingent MUST ab 99 % Auslastung im laufenden Fenster als verbraucht gelten. Ein Fenster mit Reset in der Vergangenheit MUST als neues Fenster mit 0 % zählen. Bei Gleichstand MUST die Reihenfolge nach E-Mail (sonst Name) eindeutig sein.

#### Scenario: Fenster des aktiven Abos voll
- **WHEN** das aktive Abo sein 5-h-Limit erreicht hat, ein Abo mit vollem 5-h-Fenster in 1 h zurückgesetzt wird und ein anderes mit 60 % Auslastung frei ist
- **THEN** steht nach dem aktiven Abo zuerst das freie, danach das volle.

#### Scenario: Niedrigste Woche zuerst
- **WHEN** zwei Abos ein freies 5-h-Fenster haben, eines mit 40 % und eines mit 70 % Wochenauslastung
- **THEN** steht das mit 40 % vorn.

#### Scenario: Woche bei 99 %
- **WHEN** ein Abo 99 % der Woche verbraucht hat und ein anderes ein volles 5-h-Fenster bei 50 % Woche
- **THEN** steht das Abo mit 99 % hinter dem mit vollem 5-h-Fenster.

#### Scenario: Wochenkontingent verbraucht
- **WHEN** ein Abo 100 % der Woche verbraucht hat
- **THEN** steht es hinter allen Abos mit Wochenkontingent.

### Requirement: Manueller Wechsel bei vollem Abo

Hat das aktive Abo im laufenden 5-h-Fenster 100 % oder im laufenden Wochenfenster mindestens 99 % erreicht, SHALL die Übersicht seine Karte rot umranden und „Limit erreicht“ zeigen. Das erste Abo in der Reihenfolge der Übersicht mit Nutzungsdaten, ohne erreichtes Limit, SHALL eine blau blinkende Umrandung, den Hinweis „als Nächstes“ und den Knopf „Account wechseln“ bekommen. Bei `prefers-reduced-motion` MUST die Umrandung ruhig bleiben. Der Wechsel MUST erst auf Knopfdruck über `POST /api/accounts/{id}/activate` erfolgen; danach SHALL die Übersicht das Ergebnis melden und neu laden. Hat das aktive Abo noch Luft oder ist kein Abo frei, MUST weder Signal noch Knopf erscheinen.

#### Scenario: Aktives Abo voll
- **WHEN** das aktive Abo A 100 % im 5-h-Fenster meldet, B ebenfalls voll ist und C 30 % meldet
- **THEN** ist A rot umrandet, C blau blinkend mit Knopf „Account wechseln“, und tollgate wechselt nicht von selbst.

#### Scenario: Wechsel auf Knopfdruck
- **WHEN** der Betreiber bei C „Account wechseln“ drückt
- **THEN** wird C aktiviert, die Übersicht meldet „C ist jetzt aktiv.“, und der Knopf verschwindet.

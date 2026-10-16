## ADDED Requirements

### Requirement: Manueller Wechsel bei vollem Abo

Hat das aktive Abo im laufenden 5-h- oder Wochenfenster 100 % erreicht, SHALL die Übersicht seine Karte rot umranden und „Limit erreicht“ zeigen. Das erste Abo in der Reihenfolge der Übersicht mit Nutzungsdaten, ohne erreichtes Limit, SHALL eine blau blinkende Umrandung, den Hinweis „als Nächstes“ und den Knopf „Account wechseln“ bekommen. Bei `prefers-reduced-motion` MUST die Umrandung ruhig bleiben. Der Wechsel MUST erst auf Knopfdruck über `POST /api/accounts/{id}/activate` erfolgen; danach SHALL die Übersicht das Ergebnis melden und neu laden. Hat das aktive Abo noch Luft oder ist kein Abo frei, MUST weder Signal noch Knopf erscheinen.

#### Scenario: Aktives Abo voll
- **WHEN** das aktive Abo A 100 % im 5-h-Fenster meldet, B ebenfalls voll ist und C 30 % meldet
- **THEN** ist A rot umrandet, C blau blinkend mit Knopf „Account wechseln“, und tollgate wechselt nicht von selbst.

#### Scenario: Wechsel auf Knopfdruck
- **WHEN** der Betreiber bei C „Account wechseln“ drückt
- **THEN** wird C aktiviert, die Übersicht meldet „C ist jetzt aktiv.“, und der Knopf verschwindet.

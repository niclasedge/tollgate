## ADDED Requirements

### Requirement: Schnellstes Füllen des 5-h-Fensters

`GET /api/performance` SHALL `fastest_fill` liefern. Ein Fenster ist die Menge der Traces eines Accounts mit demselben `five_hour_reset` im Zeitraum. Je Fenster gilt: erster beobachteter Stand (früheste Zeile) bis Höchststand (früheste Zeile mit dem höchsten Wert); Anstieg = Höchststand − erster Stand. Fenster mit weniger als 300 Promille Anstieg MUST NOT zählen. Hochgerechnete Zeit bis 100 % = beobachtete Dauer × 1000 / Anstieg. Gewinner ist das Fenster mit der kürzesten hochgerechneten Zeit; bei Gleichstand das ältere Fenster, dann die kleinere Account-ID. Anfragen sind die `/v1/messages`-Traces des Accounts zwischen erstem Stand und Höchststand (beide eingeschlossen), Ø Anfragen/min auf eine Nachkommastelle gerundet. Ohne zählendes Fenster MUST `fastest_fill` `null` sein. Die Berechnung MUST deterministisch sein.

#### Scenario: Volles Fenster
- **WHEN** ein Fenster von 0 auf 1000 Promille in 100 min mit 250 Anfragen steigt
- **THEN** ist die hochgerechnete Zeit 100 min, und Ø Anfragen/min ist 2,5.

#### Scenario: Teilweise beobachtetes Fenster
- **WHEN** ein Fenster von 200 auf 700 Promille in 60 min steigt
- **THEN** ist die hochgerechnete Zeit 120 min.

#### Scenario: Zu kleiner Anstieg
- **WHEN** alle Fenster weniger als 300 Promille ansteigen
- **THEN** ist `fastest_fill` `null`.

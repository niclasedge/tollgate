## ADDED Requirements

### Requirement: Abo-Karten auf der Übersicht
Ist die Account-Verwaltung verfügbar (`account_management`), SHALL die Startseite statt der einzelnen Limit-Karte je gespeichertem Abo dieselbe Abo-Karte wie der Bereich Accounts zeigen, ohne Aktionen und das aktive Abo zuerst. Ohne Account-Verwaltung MUST die bisherige Limit-Karte bleiben.

#### Scenario: Zwei Abos
- **WHEN** `/api/status` `account_management: true` liefert und zwei Abos gespeichert sind
- **THEN** zeigt die Übersicht zwei Abo-Karten mit 5-Stunden- und Wochenbalken, das aktive zuerst, und keinen Knopf „Aktivieren“

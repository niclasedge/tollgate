## ADDED Requirements

### Requirement: Einschätzung der Abo-Kapazität

Kosten & Performance SHALL eine Karte „Einschätzung“ zeigen. Bedarf pro Woche = `fills_per_day` × 7; Kapazität pro Woche = `accounts` × `windows_per_week`. Ein Fortschrittsbalken MUST den Bedarf als Anteil der Kapazität zeigen (über 100 % voll gefüllt), dazu das Urteil „reicht“ (unter 80 %), „knapp“ (80 bis 100 %) oder „reicht nicht“ (über 100 %) als Text mit rundem Statusindikator und, falls bekannt, `free_this_week` als „diese Woche noch … frei“. Kennzahlen MUST mit großer Zahl und kleiner Zeile erscheinen: Ø 5-h-Fenster je Wochenlimit (mit Zahl der Stichproben-Wochen und Preis eines vollen Fensters), gefüllte 5-h-Fenster pro Tag, Bedarf pro Woche, Kapazität mit Zahl der Accounts. Fehlt `windows_per_week` oder `fills_per_day`, MUST die Karte statt Balken und Urteil einen Hinweis zeigen, welche Messung noch fehlt.

#### Scenario: Accounts reichen nicht
- **WHEN** `fills_per_day` 5, `windows_per_week` 8 und `accounts` 4 sind
- **THEN** zeigt die Karte Bedarf 35 und Kapazität 32 pro Woche, den Balken voll und „4 Accounts reichen nicht“.

#### Scenario: Noch zu wenig gemessen
- **WHEN** `window_cost_nano_usd` `null` ist
- **THEN** zeigt die Karte keinen Balken und den Hinweis, dass noch kein 5-h-Fenster mit 30 Punkten Anstieg gemessen ist.

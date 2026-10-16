## ADDED Requirements

### Requirement: Einschätzung der Abo-Kapazität

`GET /api/performance` SHALL `capacity` liefern, deterministisch berechnet aus den Traces mit Account der letzten 8 Tage (mindestens des Zeitraums) und dem zuletzt bekannten Verbrauch der Accounts. Der Verbrauch wird nur gelesen, nie bei Anthropic abgefragt.
- Ein Fenster ist wie bei `fastest_fill` die Menge der Zeilen eines Accounts mit demselben 5-h-Reset; Anstieg = Höchststand − erster Stand.
- `fills_per_day` = Σ Anstieg der Fenster, die im Zeitraum beginnen, / 1000 / gemessene Tage; zwei Nachkommastellen. `measured_hours` (eine Nachkommastelle) = Abstand zwischen der ersten und der letzten Zeile mit 5-h-Werten im Zeitraum. Unter einer gemessenen Stunde MUST `fills_per_day` `null` sein.
- `window_cost_nano_usd` = Listenpreis der Anfragen nach der ersten bis einschließlich der Höchststand-Zeile, summiert über die Fenster mit mindestens 300 Promille Anstieg, × 1000 / Σ ihres Anstiegs. Ohne solches Fenster MUST der Wert `null` sein. `sample_windows` ist die Zahl dieser Fenster.
- `windows_per_week` = Σ (Listenpreis eines Accounts seit Beginn seines Wochenfensters, also Reset − 7 Tage, bis zur Beobachtung / `window_cost_nano_usd`) / Σ Wochenverbrauch, über die Accounts mit mindestens 25 % Wochenverbrauch, deren Wochenfenster die Traces ganz abdecken; eine Nachkommastelle. Ohne solchen Account MUST der Wert `null` sein. `sample_accounts` ist die Zahl dieser Accounts.
- `accounts` ist die Zahl der Accounts. `free_this_week` = Σ über die Accounts mit laufendem Wochenfenster von (1 − Wochenverbrauch, nicht unter 0) × `windows_per_week`; eine Nachkommastelle, sonst `null`.

#### Scenario: Wochenlimit aus den Kosten
- **WHEN** ein volles 5-h-Fenster 8 Mio. Nano-USD kostet und ein Account mit 50 % Wochenverbrauch seit Beginn seines Wochenfensters 40 Mio. Nano-USD verbraucht hat
- **THEN** ist `windows_per_week` 10,0 und `sample_accounts` 1.

#### Scenario: Wochenfenster nicht abgedeckt
- **WHEN** das Wochenfenster eines Accounts vor dem ersten Trace beginnt
- **THEN** zählt er nicht für `windows_per_week`, aber für `free_this_week`.

#### Scenario: Kein Fenster mit 30 Punkten Anstieg
- **WHEN** kein Fenster um mindestens 300 Promille steigt
- **THEN** sind `window_cost_nano_usd` und `windows_per_week` `null`, `fills_per_day` ist trotzdem gesetzt.

## MODIFIED Requirements

### Requirement: Kostenbericht

`GET /api/costs?days=N` SHALL abgeschlossene Traces der letzten N UTC-Tage (1 bis 366, Default 30) nach Tag, Client und Modell zusammenfassen: Anzahl, Usage, Schätzung, Ersparnis, Kosten nach Listenpreis und die Cache-Ersparnis `cache_savings_nano_usd`. Für Modelle ohne Preis MUST `cost_nano_usd` und `cache_savings_nano_usd` null sein. Das Feld `hours` SHALL für die letzten min(N, 8) UTC-Tage je UTC-Stunde, Account und Modell Anzahl, Usage und Kosten liefern, sortiert nach Stunde, Account und Modell; Stunden ohne Trace MUST fehlen, ohne Traces MUST `hours` eine leere Liste sein. `GET /api/performance` SHALL je Modell `hours` mit Median Tokens/s und Anzahl je UTC-Stunde der letzten höchstens 7 Tage liefern, nur für Stunden mit beurteilbaren Werten. Der Bericht MUST nur Zahlen und Kennungen enthalten und für gleiche Daten gleich ausfallen.

#### Scenario: Zwei Modelle an einem Tag
- **WHEN** ein Client an einem Tag je eine Anfrage an `claude-opus-5-5` und an ein unbekanntes Modell stellt
- **THEN** enthält der Bericht zwei Zeilen; die Opus-Zeile trägt Kosten und Cache-Ersparnis, die andere `null`.

#### Scenario: Stunden für 7 Tage
- **WHEN** `days=7` abgefragt wird und Anfragen um 01:10 und 01:50 UTC sowie um 02:05 UTC liegen
- **THEN** enthält `hours` je eine Zeile für `…T01:00:00Z` mit zwei Anfragen und für `…T02:00:00Z` mit einer.

#### Scenario: Stunden decken ein Wochenfenster
- **WHEN** `days=14` abgefragt wird und Anfragen vor 7 und vor 10 Tagen liegen
- **THEN** enthält `rows` beide Tage, `hours` aber nur die Stunden von vor 7 Tagen.

#### Scenario: Stunden für 30 Tage
- **WHEN** `days=30` abgefragt wird und Anfragen vor 10 Tagen und gestern liegen
- **THEN** enthält `rows` beide Tage, `hours` aber nur die Stunden von gestern.

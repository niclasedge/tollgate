## ADDED Requirements

### Requirement: Performance-Bericht

tollgate SHALL `GET /api/performance?days=N` (1 bis 90, Default 7, 1 = heutiger UTC-Tag) hinter dem Admin-Token bereitstellen. Die Antwort SHALL je Modell Anzahl, Zahl langsamer und beurteilter Anfragen, TTFT, Gesamtdauer und tollgate-Eigenanteil als min, median, p90 und max in Mikrosekunden, Ausgabe-Tokens pro Sekunde als Verteilung und einen Verlauf je UTC-Tag enthalten, dazu die Redactor-Kosten (oder `null`) und je Modell und Variante den neuesten Speedtest. Basis MUST abgeschlossene `messages`-Traces mit Status 200 bis 299 ohne Speedtest-Client sein. Ein ungültiges `days` MUST mit 400 abgelehnt werden. Die Antwort MUST NOT Inhalte enthalten.

#### Scenario: Zwei Modelle
- **WHEN** heute drei Anfragen an Modell A und eine an Modell B abgeschlossen wurden
- **THEN** liefert `days=1` zuerst A mit 3 Anfragen und dann B mit 1 Anfrage, jeweils mit min, median, p90 und max.

#### Scenario: Speedtest nicht in der Statistik
- **WHEN** ein Speedtest gelaufen ist
- **THEN** zählt er nicht in den Modellzeilen, erscheint aber unter `speedtests`.

#### Scenario: Ohne Admin-Token
- **WHEN** `/api/performance` ohne oder mit einem Client-Key aufgerufen wird
- **THEN** antwortet tollgate mit 401.

### Requirement: Speedtest-Endpunkt

tollgate SHALL `POST /api/speedtest` mit `{"models": [...], "confirm": true}` hinter dem Admin-Token bereitstellen und den Speedtest nach der `performance`-Spec ausführen. Ohne `confirm: true` MUST tollgate mit 400 und dem Hinweis auf das Abo-Kontingent antworten, ohne den Upstream aufzurufen. Weniger als 1 oder mehr als 4 Modelle oder ein ungültiger Modellname MUST mit 400 abgelehnt werden; läuft bereits ein Speedtest, MUST die Antwort 409 sein.

#### Scenario: Ohne Bestätigung
- **WHEN** der Admin `{"models": ["claude-opus-5-5"]}` sendet
- **THEN** antwortet tollgate mit 400, und der Upstream erhält keine Anfrage.

#### Scenario: Erfolgreicher Lauf
- **WHEN** der Admin ein Modell mit Bestätigung testet
- **THEN** enthält die Antwort zwei Ergebnisse (ohne und mit Redact) mit TTFT, Dauer, Tokens pro Sekunde, Trace-ID und dem üblichen Median.

#### Scenario: Paralleler Aufruf
- **WHEN** ein zweiter Speedtest startet, während der erste läuft
- **THEN** antwortet tollgate auf den zweiten mit 409.

### Requirement: Performance-Felder in Traces

`TraceSummary` in `GET /api/traces` und im `trace`-Event SHALL `ttft_ms`, `output_tps`, `tollgate_ms` (je Zahl oder `null`) und `perf` enthalten. `perf` MUST `null` sein, wenn kein Urteil gespeichert ist, sonst `slow` sowie für TTFT und Gesamtdauer je Urteil, Klasse, Zahl der Vergleichswerte, Median, p90 und gemessenen Wert in Millisekunden.

#### Scenario: Langsame Anfrage in der Liste
- **WHEN** eine Anfrage mit Urteil `langsam` für die Gesamtdauer gespeichert ist
- **THEN** liefert `/api/traces` für sie `perf.slow: true` und unter `perf.duration` Median, p90, Vergleichswerte und Wert.

#### Scenario: Alte Anfrage
- **WHEN** ein Trace vor der Migration entstand
- **THEN** sind `ttft_ms`, `output_tps` und `perf` `null`.

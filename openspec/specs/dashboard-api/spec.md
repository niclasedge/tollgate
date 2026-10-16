# dashboard-api Specification

## Purpose
Admin-geschützte HTTP-API unter `/api`, über die das Frontend Anfragen (Liste und Live-Events per SSE), Kosten, Clients und den Zustand von tollgate, CLIProxyAPI und Accounts abruft.

## Requirements

### Requirement: Dashboard-API hinter dem Admin-Token

tollgate SHALL unter `/api/` die Routen `GET /api/traces`, `GET /api/traces/events`, `GET /api/costs` und `GET /api/status` bereitstellen. Jede Route MUST das Admin-Token verlangen und Client-Keys ablehnen. Antworten MUST NOT Request-Inhalte, Keys oder Tokens enthalten.

#### Scenario: Ohne Admin-Token
- **WHEN** ein Aufruf von `/api/traces` ohne oder mit einem Client-Key erfolgt
- **THEN** antwortet tollgate mit 401, und der Upstream erhält keine Anfrage.

#### Scenario: Traces seitenweise
- **WHEN** der Admin `GET /api/traces?limit=2` und danach mit dem gelieferten Cursor aufruft
- **THEN** erhält er die Traces neueste zuerst, ohne Lücke und ohne Doppelte, mit Client-Name, Usage und Kosten.

### Requirement: Live-Events

`GET /api/traces/events` SHALL ein SSE-Stream sein, der für jeden gestarteten und jeden abgeschlossenen Trace ein Event `trace` sendet. Ein langsamer oder hängender Abonnent MUST NOT den Proxy verzögern.

#### Scenario: Anfrage erscheint live
- **WHEN** ein Admin den Stream abonniert und danach ein Client eine Anfrage stellt
- **THEN** erhält der Admin ein Event mit Status 0 und eines mit dem Endstatus und der Usage.

#### Scenario: Hängender Abonnent
- **WHEN** ein Abonnent keine Daten liest
- **THEN** werden Anfragen weiter ohne Verzögerung beantwortet.

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

### Requirement: Status und aktiver Account

`GET /api/status` SHALL die Health von tollgate und CLIProxyAPI sowie die Accounts aus dem auth-dir von CLIProxyAPI liefern, gelesen über die Management-API mit dem Key aus `upstream.management_key_file`. Ist der Key nicht konfiguriert oder CLIProxyAPI nicht erreichbar, MUST die Antwort trotzdem 200 sein und den Zustand in `message` beschreiben.

#### Scenario: CLIProxyAPI nicht erreichbar
- **WHEN** der Upstream nicht antwortet
- **THEN** meldet `cliproxyapi.ok` false mit einer Meldung, und `tollgate.ok` bleibt true.

### Requirement: Abo-Limits im Status

`GET /api/status` SHALL das Feld `limits` liefern: `null` ohne Messung, sonst `status`, `reset`, `representative_claim`, `observed_at` sowie `five_hour` und `seven_day` mit je `utilization` (Anteil 0..1 oder `null`), `reset` und `status`. tollgate MUST die Werte nur aus den Headern weitergeleiteter Antworten nehmen und MUST NOT dafür eigene Aufrufe an Anthropic machen.

#### Scenario: Nach einer Anfrage
- **WHEN** eine weitergeleitete Antwort Unified-Header trug
- **THEN** enthält `/api/status` diese Werte und den Zeitpunkt der Messung in RFC 3339.

#### Scenario: Noch keine Anfrage
- **WHEN** seit dem Start keine Antwort mit Unified-Headern kam
- **THEN** ist `limits` `null`.

### Requirement: Trace-Detail mit Stufen

Die Dashboard-API SHALL unter `GET /api/traces/{id}` hinter dem Admin-Token eine Anfrage mit ihrer Zusammenfassung und allen gespeicherten Stufen in Pipeline-Reihenfolge liefern: Name, Dauer, Bytes und Tokens vorher/nachher, beide Hashes, Konfigurationsversion, redigierter Diff und Treffer (Offset, Länge, Typ, Regel, Platzhalter). Eine unbekannte ID MUST mit 404 beantwortet werden. Die Antwort MUST NOT Klartext-Werte enthalten.

#### Scenario: Detail einer redigierten Anfrage
- **WHEN** eine Anfrage mit einer E-Mail-Adresse durch Intake und Redact lief und das Detail abgerufen wird
- **THEN** enthält die Antwort beide Stufen mit Diff und einem Treffer `email` mit Platzhalter `<redacted:email_1>`, aber nicht die E-Mail-Adresse.

#### Scenario: Unbekannte Anfrage
- **WHEN** `GET /api/traces/gibtsnicht` abgerufen wird
- **THEN** antwortet die API mit 404.

### Requirement: Letzte Redactions

Die Dashboard-API SHALL unter `GET /api/redactions?limit=N` (1 bis 50, Standard 10) hinter dem Admin-Token die letzten Treffer aus den Traces liefern, neueste Anfrage zuerst: Zeit, Trace-ID, Client-ID und -Name, Stufe, Typ, Regel und Platzhalter. Die Antwort MUST NOT Werte, Bodies oder Wortlisten-Einträge enthalten.

#### Scenario: Zehn neueste Treffer
- **WHEN** mehrere Anfragen mit zusammen mehr als zehn Treffern gespeichert sind
- **THEN** liefert `/api/redactions` genau zehn Treffer, beginnend mit der neuesten Anfrage.

#### Scenario: Kein Klartext
- **WHEN** eine Anfrage mit einem bekannten Geheimnis durch den Proxy lief
- **THEN** enthält die Antwort von `/api/redactions` den Platzhalter und den Typ, aber das Geheimnis nicht.

#### Scenario: Ungültiges Limit
- **WHEN** `limit=0` oder `limit=51` übergeben wird
- **THEN** antwortet die API mit 400.

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

### Requirement: Maske in Trace-Detail und letzten Redactions
`GET /api/traces/{id}` SHALL je Treffer und `GET /api/redactions` SHALL je Redaction das Feld `mask` liefern, bei älteren Traces `""`. Die Antworten MUST NOT Klartext-Werte enthalten.

#### Scenario: Maske ohne Klartext
- **WHEN** eine Anfrage mit einer erfundenen E-Mail-Adresse durch den Proxy lief
- **THEN** enthalten Trace-Detail und `/api/redactions` zum Platzhalter eine nicht leere Maske, aber weder die Adresse noch einen anderen Kanarienwert

### Requirement: Tokens/s-Urteil in Traces

`TraceSummary` SHALL ein Feld `tps` mit Urteil, Klasse, Anzahl Vergleichswerte, Tokens/s mit einer Nachkommastelle, p10, Median, p90 und Abweichung in Promille tragen. Ohne gespeichertes Urteil MUST `tps` `null` sein. Das `trace`-Event nach dem Abschluss MUST das Urteil bereits enthalten.

#### Scenario: Urteil im Event
- **WHEN** eine Stream-Anfrage abgeschlossen und beurteilt wird
- **THEN** enthält das `trace`-Event `tps.verdict` und `tps.median`.

#### Scenario: Alter Trace
- **WHEN** ein Trace vor der Migration abgeschlossen wurde
- **THEN** ist `tps` `null`.

### Requirement: Einfluss und Trend im Performance-Bericht

`GET /api/performance` SHALL je Modell ein Feld `influence` mit allen 12 Zellen aus Eingabe- und Cache-Klasse (sortiert nach Eingabe-, dann Cache-Klasse) und den Kernaussagen liefern. Jeder Tag im Verlauf SHALL zusätzlich den Median Tokens/s und den Median Cache-Anteil in Promille tragen, `null` ohne Werte. Speedtest-Traces MUST NOT eingehen.

#### Scenario: Leere Zellen
- **WHEN** ein Modell nur Anfragen der Klasse `i0` hat
- **THEN** enthält `influence.cells` trotzdem 12 Zellen, die übrigen mit `samples: 0`.

#### Scenario: Trend je Tag
- **WHEN** an einem Tag drei Stream-Anfragen mit 40, 50 und 60 Tokens/s liegen
- **THEN** meldet dieser Tag einen Median von 50 Tokens/s.

### Requirement: Anfragen der letzten fünf Minuten
Die Dashboard-API SHALL unter `GET /api/calls` hinter dem Admin-Token aus dem Store liefern: die Zahl der Traces im aktuellen Fenster [jetzt−5 min, jetzt), die Summe der Traces in den 10 Fenstern davor [jetzt−55 min, jetzt−5 min), deren Durchschnitt je Fenster mit einer Nachkommastelle und die Einordnung `above`, `below` oder `equal` des aktuellen Fensters gegen diesen Durchschnitt. Leere Fenster MUST als 0 zählen. Jeder Trace zählt, gleich welcher Endpunkt oder Status. Die Antwort MUST NOT Inhalte, Modelle oder Clients enthalten.

#### Scenario: Mehr als üblich
- **WHEN** in den letzten 5 Minuten 4 Anfragen und in den 50 Minuten davor 20 Anfragen eingingen
- **THEN** liefert `/api/calls` `current: 4`, `previous_total: 20`, `average: 2` und `trend: "above"`.

#### Scenario: Fenstergrenzen
- **WHEN** Traces genau bei jetzt−5 min, genau bei jetzt−55 min, kurz vor jetzt−55 min und genau bei jetzt liegen
- **THEN** zählt der erste zum aktuellen Fenster, der zweite zu den Vorfenstern und die beiden anderen gar nicht.

#### Scenario: Keine Anfragen
- **WHEN** in der letzten Stunde keine Anfrage einging
- **THEN** liefert `/api/calls` `current: 0`, `previous_total: 0`, `average: 0` und `trend: "equal"`.

#### Scenario: Nur Admin
- **WHEN** `/api/calls` ohne Admin-Token oder mit einem Client-Key abgerufen wird
- **THEN** antwortet die API mit 401.

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

## ADDED Requirements

### Requirement: Performance-Ansicht

Das Frontend SHALL einen Bereich Performance mit Zeitraum-Wahl (Heute, 7 Tage, 30 Tage) zeigen: je Modell Anzahl, Anteil langsamer Anfragen, TTFT und Gesamtdauer als min / median / p90 / max, Ausgabe-Tokens pro Sekunde und aufklappbar einen dezenten Verlauf je Tag sowie den tollgate-Eigenanteil. Darunter SHALL eine Karte Redactor-Kosten (Redact-Stufe und Rückweg je median, p90, max in ms, Anteil an der Gesamtdauer, Bytes, Treffer, zusätzliche Tokens) mit dem Hinweis stehen, dass deterministische Platzhalter den Prompt-Cache nicht ändern und zusätzliche Tokens geschätzt sind. Fehlen Daten, MUST „Noch keine Daten“ erscheinen statt geratener Werte.

#### Scenario: Tabelle je Modell
- **WHEN** `/api/performance` ein Modell mit TTFT-Median 1 200 000 µs liefert
- **THEN** zeigt die Zeile dieses Modells für TTFT min, median „1,2 s“, p90 und max.

#### Scenario: Ohne Redactor-Daten
- **WHEN** `redactor` `null` ist
- **THEN** zeigt die Karte Redactor-Kosten „Noch keine Daten“.

### Requirement: Langsame Anfragen rot

Live (Liste und Inline-Detail) und die letzten Anfragen der Startseite SHALL eine Anfrage mit `perf.slow` mit rotem Statuspunkt zeigen, dessen Beschriftung die Erklärung trägt, z. B. „Gesamtdauer: erwartet ~4,2 s, gebraucht 9,8 s“. Das Inline-Detail SHALL zusätzlich TTFT, Tokens pro Sekunde, tollgate-Eigenanteil und die Erklärung als Text zeigen. Eine laufende oder fehlgeschlagene Anfrage MUST ihren bisherigen Punkt behalten. Ohne Urteil MUST das Detail „noch zu wenig Vergleichswerte“ zeigen und der Punkt MUST NOT rot sein.

#### Scenario: Verzögerte Anfrage
- **WHEN** ein `trace`-Event mit `perf.slow: true`, Median 4,2 s und Wert 9,8 s eintrifft
- **THEN** zeigt Live die Anfrage mit rotem Punkt und dem Text „erwartet ~4,2 s, gebraucht 9,8 s“.

#### Scenario: Zu wenig Daten
- **WHEN** `perf` eines abgeschlossenen Traces `unbekannt` meldet
- **THEN** ist der Punkt nicht rot, und das Detail zeigt „noch zu wenig Vergleichswerte“.

### Requirement: Speedtest im Frontend

Die Performance-Ansicht SHALL einen Knopf „Speed testen“ mit Modellauswahl bieten. Vor dem Start MUST ein Dialog bestätigen lassen, dass der Test Abo-Kontingent verbraucht; erst danach wird `POST /api/speedtest` mit `confirm: true` gesendet. Das Ergebnis SHALL je Modell ohne und mit Redact TTFT, Dauer und Tokens pro Sekunde neben dem üblichen Median zeigen.

#### Scenario: Abbrechen
- **WHEN** der Nutzer den Bestätigungsdialog abbricht
- **THEN** sendet das Frontend keine Anfrage.

#### Scenario: Ergebnis
- **WHEN** die API zwei Ergebnisse für ein Modell liefert
- **THEN** zeigt die Karte beide Varianten mit TTFT, Dauer und Tokens pro Sekunde und den üblichen Median daneben.

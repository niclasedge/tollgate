## ADDED Requirements

### Requirement: Abgeleitete Messwerte

tollgate SHALL je abgeschlossener Anfrage die Ausgabe-Tokens pro Sekunde als `usage_output / ((duration_us − ttft_us) / 10⁶)` mit einer Nachkommastelle und den tollgate-Eigenanteil als Summe aller Stufendauern plus `return_us` berechnen. Tokens pro Sekunde MUST `null` sein, wenn die Antwort kein SSE war, `usage_output` kleiner 1 ist, TTFT fehlt oder `duration_us <= ttft_us` gilt.

#### Scenario: Gestreamte Antwort
- **WHEN** eine SSE-Anfrage `duration_us = 5 000 000`, `ttft_us = 1 000 000` und `usage_output = 200` hat
- **THEN** ist `output_tps` 50,0.

#### Scenario: Nicht gestreamte Antwort
- **WHEN** eine JSON-Antwort abgeschlossen wird
- **THEN** ist `output_tps` `null`.

#### Scenario: Eigenanteil ohne Rückweg
- **WHEN** die Stufen 300 µs und 1 200 µs dauern und `return_us` `NULL` ist
- **THEN** beträgt der tollgate-Eigenanteil 1 500 µs.

### Requirement: Größenklassen und Vergleichswerte

tollgate SHALL TTFT je Modell und Eingabe-Klasse und die Gesamtdauer je Modell, Eingabe- und Ausgabe-Klasse vergleichen. Eingabe zählt `usage_input + usage_cache_read + usage_cache_creation` mit den Grenzen 8 000, 32 000 und 128 000 (`i0` bis `i3`), Ausgabe `usage_output` mit den Grenzen 256, 1 024 und 4 096 (`o0` bis `o3`). Hat die Dauer-Klasse weniger als 20 Vergleichswerte, MUST genau einmal auf Modell und Ausgabe-Klasse zurückgefallen werden. Vergleichswerte SHALL nur Traces desselben Modells aus `[Start − 14 Tage, Start)` sein, mit Endpoint `messages`, Status 200 bis 299, leerem `error_kind`, vorhandener Usage und Messgröße, ohne den Speedtest-Client, davon die neuesten 500.

#### Scenario: Klassengrenze
- **WHEN** eine Anfrage 7 999 Eingabe-Tokens inklusive Cache hat und eine zweite 8 000
- **THEN** liegt die erste in `i0` und die zweite in `i1`.

#### Scenario: Rückfall der Dauer-Klasse
- **WHEN** die Klasse `i2o1` 12 und die Klasse `o1` 40 Vergleichswerte hat
- **THEN** wird die Gesamtdauer gegen `o1` beurteilt, und die gespeicherte Klasse lautet `o1`.

#### Scenario: Spätere Anfragen zählen nicht
- **WHEN** eine Anfrage beurteilt wird und es Traces mit späterem Start gibt
- **THEN** gehören diese nicht zu den Vergleichswerten.

### Requirement: Urteil „langsam“

Für TTFT und Gesamtdauer SHALL tollgate Median und p90 der Vergleichswerte nach der Nearest-Rank-Methode (`x[⌈q·n⌉ − 1]` auf sortierten Ganzzahlen) bilden. Eine Messgröße MUST genau dann `langsam` sein, wenn der Wert größer als p90, mindestens 1,5 × Median und mindestens 1 s über dem Median liegt; bei weniger als 20 Vergleichswerten MUST sie `unbekannt` sein, sonst `normal`. Eine Anfrage ist langsam, wenn eine ihrer Messgrößen `langsam` ist. Die Berechnung MUST deterministisch sein, und ein gespeichertes Urteil MUST NOT sich nachträglich ändern.

#### Scenario: Künstlich verzögerte Anfrage
- **WHEN** 20 Vergleichswerte mit Median 4,2 s und p90 6,1 s vorliegen und eine Anfrage 9,8 s braucht
- **THEN** ist die Gesamtdauer `langsam` mit Erwartung 4,2 s, p90 6,1 s und 20 Vergleichswerten.

#### Scenario: Nur über p90
- **WHEN** der Median 4,0 s und p90 5,0 s beträgt und die Anfrage 5,5 s braucht
- **THEN** ist das Urteil `normal`, weil 5,5 s unter 1,5 × Median liegt.

#### Scenario: Kleine Werte
- **WHEN** der TTFT-Median 0,4 s und p90 0,5 s beträgt und die Anfrage 0,9 s braucht
- **THEN** ist das Urteil `normal`, weil der Abstand zum Median unter 1 s liegt.

#### Scenario: Zu wenig Daten
- **WHEN** nur 19 Vergleichswerte vorliegen
- **THEN** ist das Urteil `unbekannt`, und es werden keine Median- und p90-Werte gespeichert.

#### Scenario: Reihenfolge egal
- **WHEN** dieselben Vergleichswerte in anderer Reihenfolge übergeben werden
- **THEN** sind Median, p90 und Urteil identisch.

### Requirement: Redactor-Kosten

tollgate SHALL für Anfragen mit der Stufe `redact` im Zeitraum die Dauer der Stufe und des Rückwegs als min, median, p90 und max, den Median des Anteils (Stufe + Rückweg) an der Gesamtdauer, Bytes und Treffer der Stufe als Bezugsgrößen sowie die zusätzlichen Tokens als `tokens_out − tokens_in` der Stufe (Summe und Median) liefern. Ohne solche Anfragen MUST der Bericht `null` sein. Die Werte MUST nur aus Zahlen des Stores stammen.

#### Scenario: Zwei redigierte Anfragen
- **WHEN** zwei Anfragen mit Stufe `redact` (400 µs und 600 µs, je +3 Tokens) im Zeitraum liegen
- **THEN** meldet der Bericht 2 Anfragen, max 600 µs und 6 zusätzliche Tokens.

#### Scenario: Redact ausgeschaltet
- **WHEN** im Zeitraum keine Anfrage die Stufe `redact` durchlief
- **THEN** ist `redactor` `null`.

### Requirement: Speedtest über die volle Pipeline

tollgate SHALL einen Speedtest anbieten, der je gewähltem Modell eine feste Testanfrage ohne Geheimnisse nacheinander ohne und mit Redact-Stufe über denselben Code wie eine Client-Anfrage schickt (Account-Gate, Pipeline, Forward, Meter, Trace, Urteil) und TTFT, Dauer, Tokens pro Sekunde sowie den üblichen Median des Modells liefert. Der Aufruf MUST einen reservierten internen Client nutzen, dessen Identität nur im Prozess gesetzt werden kann; das Überspringen einer Stufe MUST NOT für andere Stufen als `redact` und MUST NOT über HTTP möglich sein. Speedtest-Traces MUST NOT als Vergleichswerte oder in der Modellstatistik zählen. Der Speedtest MUST nur auf ausdrücklichen Auftrag mit Bestätigung laufen, und es darf höchstens einer gleichzeitig laufen.

#### Scenario: Mit und ohne Redact
- **WHEN** der Admin den Speedtest für ein Modell bestätigt
- **THEN** entstehen zwei Traces des Clients `tollgate:speedtest`, einer ohne und einer mit Stufe `redact`, und beide liefern TTFT, Dauer und Tokens pro Sekunde.

#### Scenario: Redact-Stufe aus
- **WHEN** `redact.enabled` false ist
- **THEN** läuft nur die Variante ohne Redact, und die Variante mit Redact meldet „Redact-Stufe aus“.

#### Scenario: Kein Weg von außen
- **WHEN** ein Client per HTTP Header oder Body sendet, die den internen Client oder das Überspringen einer Stufe nachahmen
- **THEN** wird er wie jeder Client geprüft, und alle Stufen laufen.

#### Scenario: Kein Klartext
- **WHEN** ein Speedtest gelaufen ist
- **THEN** stehen der Text der Testanfrage und die Antwort weder in der Datenbank noch im Log.

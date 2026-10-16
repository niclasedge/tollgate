## ADDED Requirements

### Requirement: Urteil Tokens pro Sekunde

tollgate SHALL je abgeschlossener SSE-Anfrage mit `usage_output ≥ 64` die Ausgabe-Tokens pro Sekunde als Ganzzahl `tps_milli = usage_output · 10⁹ / (duration_us − gen_start_us)` gegen Vergleichswerte desselben Modells und derselben Eingabe-Klasse beurteilen. `gen_start_us` MUST die Zeit bis zum Flush von `message_start` sein, nicht die TTFT, weil verdecktes Thinking vor dem ersten `content_block_delta` liegt und trotzdem als Ausgabe zählt. Vergleichswerte SHALL dieselbe Datenbasis wie das Urteil „langsam“ nutzen, zusätzlich nur SSE mit `gen_start_us` und `usage_output ≥ 64`. Bei weniger als 20 Vergleichswerten MUST genau einmal auf das Modell ohne Klasse (`*`) zurückgefallen werden. Mit p10, Median und p90 nach Nearest-Rank MUST das Urteil `langsamer` sein, wenn der Wert unter p10 und mindestens 20 % unter dem Median liegt, `schneller`, wenn er über p90 und mindestens 20 % über dem Median liegt, sonst `üblich`; bei weniger als 20 Vergleichswerten oder `usage_output < 64` MUST es `unbekannt` sein. Das Urteil MUST deterministisch sein und sich nachträglich nicht ändern.

#### Scenario: Langsamere Generierung
- **WHEN** 30 Vergleichswerte mit p10 51 000, Median 62 000 und p90 70 000 Milli-Tokens/s vorliegen und eine Anfrage 48 000 erreicht
- **THEN** ist das Urteil `langsamer` mit einer Abweichung von −225 ‰.

#### Scenario: Unter p10, aber nah am Median
- **WHEN** p10 60 000 und Median 62 000 betragen und die Anfrage 58 000 erreicht
- **THEN** ist das Urteil `üblich`, weil die Abweichung kleiner als 20 % ist.

#### Scenario: Kurze Antwort
- **WHEN** eine Anfrage nur 40 Ausgabe-Tokens hat
- **THEN** ist das Urteil `unbekannt`.

#### Scenario: Rückfall auf das Modell
- **WHEN** die Klasse `i3` 8 und das Modell insgesamt 50 Vergleichswerte hat
- **THEN** wird gegen das Modell beurteilt, und die gespeicherte Klasse lautet `*`.

#### Scenario: Verdecktes Thinking
- **WHEN** `message_start` nach 0,1 s ankommt, das erste `content_block_delta` erst nach 8,5 s, die Anfrage 8,6 s dauert und 686 Ausgabe-Tokens meldet
- **THEN** beträgt der Wert rund 81 t/s (686 / 8,5 s) und nicht über 6.000 t/s.

### Requirement: Einfluss von Eingabe und Cache

tollgate SHALL je Modell und Zeitraum eine Matrix aus Eingabe-Klasse (`i0`–`i3`) und Cache-Klasse liefern. Die Cache-Klasse ergibt sich aus `usage_cache_read · 1000 / input_total`: `c0` unter 500, `c1` 500 bis 899, `c2` ab 900. Jede der 12 Zellen MUST Anzahl sowie Median TTFT, Median Tokens/s und Median Dauer enthalten; bei weniger als 5 Werten MUST nur die Anzahl gefüllt und die Mediane `null` sein. tollgate SHALL daraus höchstens zwei Kernaussagen ableiten: die relative Änderung des TTFT-Medians von `c0` zu `c2` in der Eingabe-Klasse mit den meisten Werten, und die relative Änderung des Tokens/s-Medians von der kleinsten zur größten Eingabe-Klasse mit Werten, jeweils in Promille und nur, wenn beide Zellen mindestens 5 Werte haben.

#### Scenario: Cache-Klassengrenze
- **WHEN** eine Anfrage einen Cache-Anteil von 899 ‰ und eine zweite von 900 ‰ hat
- **THEN** liegt die erste in `c1` und die zweite in `c2`.

#### Scenario: Warmer Cache senkt TTFT
- **WHEN** in `i2` die Zelle `c0` einen TTFT-Median von 3,4 s und die Zelle `c2` von 2,0 s hat, je mit mindestens 5 Werten
- **THEN** meldet die Kernaussage `cache_ttft` für `i2` eine Änderung von −411 ‰.

#### Scenario: Dünne Zelle
- **WHEN** eine Zelle 4 Werte hat
- **THEN** enthält sie nur die Anzahl 4, und sie zählt für keine Kernaussage.

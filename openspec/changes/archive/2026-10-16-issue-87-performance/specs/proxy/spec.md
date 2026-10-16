## ADDED Requirements

### Requirement: Zeit bis zum ersten Token

Der Proxy SHALL für `/v1/messages` die Zeit vom Beginn der Anfrage (derselbe Startpunkt wie `duration_us`) bis zum ersten Token als `ttft_us` im Trace speichern: bei SSE bis das erste Event `content_block_delta` (jeder Delta-Typ) an den Client geschrieben und geflusht ist, bei JSON bis die Antwort-Header des Upstreams eingetroffen sind. Bei Status ab 400, Abbruch vor dem ersten Delta, `count_tokens` und durchgereichten Endpunkten MUST `ttft_us` `NULL` bleiben. Die Messung MUST NOT die Bytes an den Client verändern oder verzögern.

#### Scenario: Verzögerter Stream
- **WHEN** ein `httptest`-Upstream `message_start` sofort und das erste `content_block_delta` erst nach 300 ms sendet
- **THEN** liegt `ttft_us` bei mindestens 300 000, höchstens bei `duration_us`, und der Client erhält den Stream byte-gleich.

#### Scenario: JSON-Antwort
- **WHEN** ein Upstream die Header einer JSON-Antwort nach 200 ms sendet
- **THEN** liegt `ttft_us` bei mindestens 200 000.

#### Scenario: Fehlerantwort
- **WHEN** der Upstream mit 429 antwortet
- **THEN** ist `ttft_us` `NULL`.

### Requirement: Rückweg-Messwerte und Urteil beim Abschluss

Der Proxy SHALL beim Abschluss einer Anfrage `return_us` (Zeit in der Rückübersetzung, ohne Warten auf den Upstream) und `return_replacements` übernehmen, sofern ein Rückweg lief, die Bytes vor und nach jeder Stufe in den Trace schreiben und danach das Urteil nach der `performance`-Spec berechnen und speichern, bevor das Live-Event zum Abschluss gesendet wird. Ein Fehler bei Abfrage oder Speicherung MUST geloggt werden und MUST NOT die Antwort an den Client beeinflussen.

#### Scenario: Live-Event mit Urteil
- **WHEN** eine Anfrage nach 20 schnellen Vergleichsanfragen künstlich verzögert abgeschlossen wird
- **THEN** trägt das abschließende `trace`-Event das Urteil `langsam`.

#### Scenario: Store-Fehler beim Urteil
- **WHEN** die Vergleichsabfrage fehlschlägt
- **THEN** hat der Client seine vollständige Antwort, und der Trace hat kein Urteil.

## ADDED Requirements

### Requirement: Usage aus der Antwort

Für `/v1/messages` SHALL tollgate die vom Upstream gemeldete `usage` (`input_tokens`, `output_tokens`, `cache_read_input_tokens`, `cache_creation_input_tokens`) im Trace speichern, bei JSON-Antworten aus dem Feld `usage` und bei SSE aus `message_start` und `message_delta`. Die Antwort an den Client MUST byte-gleich bleiben. Fehlt `usage`, MUST der Trace ohne Usage abgeschlossen werden.

#### Scenario: Nicht gestreamte Antwort
- **WHEN** der Upstream eine JSON-Antwort mit `usage` liefert
- **THEN** erhält der Client dieselben Bytes, und der Trace enthält alle vier Werte.

#### Scenario: Gestreamte Antwort über Chunk-Grenzen
- **WHEN** der Upstream `message_start` mit Eingabe-Usage und `message_delta` mit `output_tokens` sendet, jeweils über zwei Chunks verteilt
- **THEN** ist der Stream beim Client unverändert, und der Trace enthält Eingabe und Cache aus `message_start` sowie die Ausgabe aus `message_delta`.

#### Scenario: count_tokens wird nicht gemessen
- **WHEN** ein Client `/v1/messages/count_tokens` aufruft
- **THEN** speichert der Trace keine Usage.

### Requirement: Kosten pro Modell

tollgate SHALL die Kosten einer Usage aus den Listenpreisen des Modells in Nano-USD berechnen, wobei das längste an einer Grenze endende Modell-Präfix gilt. Für ein unbekanntes Modell MUST die Berechnung „unbekannt“ melden statt eines Werts.

#### Scenario: Opus 5.5
- **WHEN** je eine Million Eingabe-, Ausgabe-, Cache-Read- und Cache-Write-Tokens für `claude-opus-5-5` bewertet werden
- **THEN** ergibt sich 29,20 USD.

#### Scenario: Unbekanntes Modell
- **WHEN** ein Modell keinem Präfix entspricht
- **THEN** meldet die Berechnung keine Kosten.

### Requirement: Schätzung und Usage im Log

tollgate SHALL vor dem Weiterleiten die Token-Schätzung und nach der Antwort Usage und Kosten loggen. Die Logzeilen MUST nur Zahlen, Modell und Trace-ID enthalten, keine Inhalte.

#### Scenario: Logzeile nach der Antwort
- **WHEN** eine Anfrage mit Usage abgeschlossen ist
- **THEN** steht im Log eine Zeile `request metered` mit Trace-ID, Modell, Status, den vier Token-Werten und, falls bekannt, `cost_nano_usd`.

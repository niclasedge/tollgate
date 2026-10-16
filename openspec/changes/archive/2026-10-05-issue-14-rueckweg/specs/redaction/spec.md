## ADDED Requirements

### Requirement: Rückübersetzung bekannter Platzhalter

Der Rückweg SHALL jeden Platzhalter, der in der Zuordnung der Anfrage (`Request.Placeholders`) steht, durch seinen Klartext ersetzen. Platzhalter ohne Eintrag in der Zuordnung MUST unverändert bleiben. In `input_json_delta` und JSON-Kontexten MUST der Klartext nach ADR-0004 JSON-escaped eingesetzt werden. Bei leerer Zuordnung MUST die Antwort Byte für Byte unverändert bleiben.

#### Scenario: Bekannter und unbekannter Platzhalter
- **WHEN** ein Text `<redacted:email_1>` und `<redacted:email_9>` enthält und nur `email_1` in der Zuordnung steht
- **THEN** wird nur `<redacted:email_1>` ersetzt.

#### Scenario: Escaping in input_json_delta
- **WHEN** der Klartext Anführungszeichen, Backslash oder Zeilenumbruch enthält und der Platzhalter in `partial_json` steht
- **THEN** ergibt das zusammengesetzte `partial_json` gültiges JSON, dessen String-Wert genau den Klartext enthält.

### Requirement: Puffer pro Content-Block im Stream

Der Rückweg SHALL im SSE-Stream pro Content-Block (`index`) einen eigenen Puffer führen. Endet ein `text_delta` oder `input_json_delta` mit einem Präfix, der noch zu einem Platzhalter werden kann, MUST dieser Rest (höchstens 49 Bytes) zurückgehalten und dem nächsten Delta desselben Blocks vorangestellt werden. Ein Delta, das dadurch leer wird, MUST trotzdem mit leerem Text weitergeleitet werden. Bei `content_block_stop` MUST ein verbleibender Rest ungeprüft als eigenes Delta vor dem Stop-Event ausgegeben werden. Die Ausgabe MUST unabhängig davon sein, wie der Upstream die Bytes auf Chunks verteilt.

#### Scenario: Platzhalter über Delta-Grenzen
- **WHEN** `<redacted:email_1>` auf drei aufeinanderfolgende `text_delta` verteilt ist
- **THEN** enthält der zusammengesetzte Text des Blocks den Klartext, und die Zahl der Delta-Events bleibt gleich.

#### Scenario: Verschränkte Blöcke
- **WHEN** Deltas der Blöcke 0 und 1 abwechselnd kommen und jeder einen Teil eines Platzhalters trägt
- **THEN** wird jeder Block nur mit seinen eigenen Deltas zusammengesetzt.

#### Scenario: Unvollständiger Rest am Blockende
- **WHEN** ein Block mit `<redac` endet
- **THEN** steht `<redac` unverändert in einem Delta vor `content_block_stop`.

#### Scenario: Chunk-Grenzen im HTTP-Stream
- **WHEN** ein Mock-Upstream (`httptest`) denselben Stream byteweise, in zufälligen und in ganzen Stücken mit Flush sendet
- **THEN** ist die Ausgabe in allen Fällen Byte für Byte gleich.

### Requirement: Thinking und unbekannte Events unverändert

`thinking_delta`, `signature_delta`, `content_block_start`, `message_start`, `message_delta`, `message_stop`, `ping`, `error`, Kommentare und unbekannte Events oder Delta-Typen SHALL Byte für Byte durchgereicht werden. In umgeschriebenen Deltas MUST sich nur der Wert von `delta.text` bzw. `delta.partial_json` ändern; alle anderen Bytes des Events bleiben erhalten.

#### Scenario: Thinking mit Platzhalter
- **WHEN** ein `thinking_delta` `<redacted:email_1>` enthält
- **THEN** wird das Event unverändert weitergeleitet.

#### Scenario: Unbekannte Felder
- **WHEN** ein `text_delta`-Event zusätzliche, unbekannte Felder trägt
- **THEN** stehen diese Felder bytegleich im umgeschriebenen Event.

### Requirement: Nicht-Streaming-Antworten

Bei einer JSON-Antwort SHALL der Rückweg nur `content[].text` von `text`-Blöcken und alle String-Werte in `content[].input` von `tool_use`-Blöcken per Byte-Splicing zurückübersetzen. Thinking-Blöcke, Signaturen und alle übrigen Felder MUST bytegleich bleiben. Ist der Body kein gültiges JSON-Objekt, MUST er unverändert bleiben.

#### Scenario: text und tool_use
- **WHEN** eine Antwort einen `thinking`-Block, einen `text`-Block und einen `tool_use`-Block mit `<redacted:email_1>` enthält
- **THEN** sind `text` und `tool_use.input` zurückübersetzt, der Thinking-Block ist bytegleich.

### Requirement: Redact und Rehydrate sind invers

Wird ein beliebiger Text redigiert und das Ergebnis über den Stream-Rückweg in beliebigen Delta-Stücken zurückübersetzt, SHALL das Ergebnis exakt dem Ausgangstext entsprechen, auch wenn der Ausgangstext selbst Platzhaltertext enthält (reservierte Nummern stehen nicht in der Zuordnung).

#### Scenario: Fuzz
- **WHEN** der Fuzz-Test beliebige Texte redigiert und den redigierten Text an beliebigen Stellen in Deltas teilt
- **THEN** ergibt der zurückübersetzte Blocktext den Ausgangstext, für `text_delta` und `input_json_delta`.

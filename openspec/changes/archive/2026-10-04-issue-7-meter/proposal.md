# Proposal

Issue: [#7 Meter: Token- und Kostenmessung](https://github.com/niclasedge/tollgate/issues/7)

Geht zurück auf GitHub-Issue #7. Abhängigkeiten #4 und #6 sind erfüllt.

## Why

tollgate speichert pro Anfrage bisher nur eine grobe Schätzung der Eingabe-Tokens. Was die Anfrage tatsächlich verbraucht hat, kennt nur die Antwort des Upstreams. Ohne diese Werte zeigen weder Logs noch die spätere Kosten-Ansicht (#10), was ein Client verbraucht und wie gut der Prompt-Cache trifft.

## What Changes

- Neues Paket `internal/meter`: liest `usage` (`input`, `output`, `cache_read`, `cache_creation`) aus nicht gestreamten JSON-Antworten und aus SSE-Streams (`message_start`, `message_delta`).
- Der Proxy beobachtet die Antwort von `/v1/messages`, ohne sie zu verändern, und schreibt die Werte mit `FinishTrace` in den Trace. Auch ein abgebrochener Stream speichert, was bis dahin gemeldet wurde.
- Kosten pro Modell: Listenpreise der Anthropic-API als Tabelle nach Modell-Präfix, Ergebnis in Nano-USD (Ganzzahl, deterministisch). Unbekannte Modelle haben keine Kosten.
- Logs: vor dem Weiterleiten die Schätzung (`debug`), nach der Antwort eine Zeile mit Usage und Kosten (`info`), nur Zahlen.
- Die Schätzheuristik `ceil(bytes/4)` wird festgelegt und begründet.

## Capabilities

### New Capabilities

- `meter`: Usage-Erfassung aus Antworten und Kosten pro Modell.

### Modified Capabilities

Keine.

## Was ausdrücklich nicht

- Keine Kosten-Ansicht und keine Aggregation nach Tag/Client/Modell: das ist #10.
- Kein Budget-Alarm: das ist Phase 4.
- Keine Aufteilung der Cache-Writes nach 5 Minuten und 1 Stunde; der Store kennt nur die Summe.

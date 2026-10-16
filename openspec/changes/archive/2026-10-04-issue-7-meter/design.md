# Design

## Wo gemessen wird

Der Proxy kopiert die Antwort weiter wie bisher. Für `/v1/messages` hängt er einen Beobachter an:

- **SSE** (`text/event-stream`): `sse.CopyObserve` reicht jedes vollständige Event an `meter.Stream.Observe`, bevor es geschrieben wird. `message_start` liefert `message.usage`, `message_delta` die kumulierten Werte. Ein vorhandenes Feld überschreibt den bisherigen Wert, ein fehlendes lässt ihn stehen.
- **JSON** (`application/json`): Eine Kopie der Antwort landet in einem Puffer mit Obergrenze 8 MiB. Größere Antworten gehen unverändert durch und bleiben ohne Usage. Der Puffer liefert nie einen Fehler und kann die Kopie zum Client nicht abbrechen.

`count_tokens` und durchgereichte Endpunkte werden nicht gemessen.

## Kosten

Die Preise stehen in `internal/meter/cost.go` als Nano-USD pro Token. Das ist dieselbe Zahl wie USD pro Million Tokens mal 1000, also `$4 / MTok = 4000`. Ganzzahlen halten Summen exakt und deterministisch.

- Gesucht wird das längste Modell-Präfix, das an einer Grenze endet (`-`, `[` oder Ende). `claude-opus-5-5` hat Vorrang vor `claude-opus-5`, und `claude-opus-40` trifft `claude-opus-4` nicht.
- Cache-Writes kosten den 5-Minuten-Preis. Bei 1-Stunden-Caching ist der Wert daher eine Untergrenze.
- Quelle: platform.claude.com/docs/en/about-claude/pricing, Stand 2026-10-04.
- Ein Preiswechsel ändert die Tabelle. Die Kosten werden aus Usage und Modell berechnet und nicht gespeichert, deshalb wirkt ein Wechsel auch rückwirkend in der Anzeige.

## Schätzheuristik

`ceil(bytes/4)` über den finalen JSON-Body:

- Die JSON-Struktur bläht die Bytezahl auf.
- Der Tokenizer ab Claude 4.7 erzeugt etwa 30 % mehr Tokens pro Text.
- Beides gleicht sich grob aus.

Die Schätzung ist ein billiger, deterministischer Hinweis im Trace. Maßgeblich ist die gespeicherte Usage, und mit beiden Werten im Trace lässt sich die Abweichung später messen. `count_tokens` als Schätzung ist verworfen: Es kostet eine zusätzliche Upstream-Anfrage pro Turn.

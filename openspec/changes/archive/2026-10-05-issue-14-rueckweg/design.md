# Design

Verbindlich ist [ADR-0006](../../../docs/adr/0006-platzhalter-sitzungen-rueckweg.md),
Abschnitt „Rückweg im Stream“. Hier steht, wie der Code die Entscheidung umsetzt.

## API (`internal/redact`)

```go
func Rehydrate(s string, mapping map[string]string) string      // Klartext einsetzen
func RehydrateJSON(s string, mapping map[string]string) string  // Klartext JSON-escaped (ADR-0004)

type Holdback struct{ /* Puffer eines Content-Blocks */ }
func NewHoldback(mapping map[string]string, json bool) *Holdback
func (h *Holdback) Write(chunk string) string  // Text, der jetzt ausgegeben werden kann
func (h *Holdback) Flush() string              // Rest, ungeprüft

func CopyRehydrated(dst io.Writer, flush func(), src io.Reader,
    mapping map[string]string, observe func(ev []byte)) (int64, error)
func RehydrateBody(body []byte, mapping map[string]string) []byte
```

`CopyRehydrated` hat dieselbe Form wie `sse.CopyObserve`; `observe` bekommt die
**unveränderten** Upstream-Events (der Meter liest nur `message_start`/`message_delta`, die
nie umgeschrieben werden). Bei leerer Zuordnung delegiert die Funktion an
`sse.CopyObserve`, die Antwort bleibt also bytegleich.

## Puffer (Holdback)

Pro Delta: Arbeitstext = Puffer + Delta. Gesucht wird das letzte `<` in den letzten 49 Bytes.
Ist der Rest ab dort ein **lebensfähiger Präfix** der Grammatik
`<redacted:TYP_N>` (ohne `>`), bleibt er im Puffer. Lebensfähig heißt: Präfix von
`<redacted:`, oder `<redacted:` gefolgt von `[a-z][a-z0-9_]{0,31}`, oder von
`TYP_` plus `[1-9][0-9]{0,5}` (auch leer) für irgendeine Zerlegung an einem `_`.
Ein vollständiger Platzhalter enthält genau ein `<` und endet mit `>`; ein lebensfähiger
Rest enthält kein `>`. Daher liegt der Schnitt nie in einem vollständigen Platzhalter, und
die Ersetzung auf dem ausgegebenen Teil findet dieselben Treffer wie auf dem ganzen Text.
Der Schnitt liegt immer vor einem `<`, also auf einer UTF-8-Zeichengrenze.

## Stream

- Events werden mit `sse.Reader` gerahmt. Nur `content_block_delta` mit genau einer
  `data:`-Zeile und `delta.type` `text_delta` bzw. `input_json_delta` wird bearbeitet.
- Die Lage des einen String-Tokens (`delta.text` bzw. `delta.partial_json`) ermittelt ein
  `jsontext.Decoder`; ersetzt wird per Byte-Splicing nur dieses Token. Ist das neue Ergebnis
  gleich dem alten Wert, wird das Original-Event bytegleich geschrieben.
- `input_json_delta`: Der dekodierte `partial_json`-Text ist selbst JSON-Text. Er wird mit
  `RehydrateJSON` (Klartext JSON-escaped) bearbeitet und anschließend für den SSE-Umschlag
  erneut escaped. Annahme (ADR-0006): Platzhalter stehen roh, nicht als `<…`.
- Puffer pro `index`. Die Puffer liegen in einer Map, die nur zum Nachschlagen dient;
  beim Leeren mehrerer Puffer wird nach `index` sortiert (deterministisch).
- Leeren:
  - `content_block_stop` für `index`: Rest als synthetisches Delta direkt davor.
  - `message_delta`, `message_stop`, `error`, nicht parsebare `content_block_delta`-Events
    und Stream-Ende: alle offenen Reste, aufsteigend nach `index`.
  - Wechselt der Delta-Typ eines Blocks (sollte nie vorkommen), wird der alte Rest zuerst als
    eigenes Delta seines Typs ausgegeben.
- Synthetisches Delta: `event: content_block_delta\ndata: {"type":"content_block_delta","index":N,"delta":{"type":"text_delta","text":"…"}}\n\n`
  (bzw. `input_json_delta`/`partial_json`). Es entsteht nur, wenn ein Block mit einem
  unvollständigen Platzhalter-Präfix endet; dann gibt es ein Event mehr als upstream.

## Nicht-Streaming

`RehydrateBody` läuft mit `jsontext.Decoder` über den Body, sammelt in `content[]` die
String-Token von `text` (Block-Typ `text`) und alle String-Werte in `input` (Block-Typ
`tool_use`, rekursiv) und ersetzt per `jsonsplice.Apply`. Da `type` nach `text`/`input`
stehen kann, wird jeder Block erst ganz gelesen und dann entschieden. Ungültiges JSON:
Body unverändert.

## Verdrahtung (Koordinator, `internal/server/server.go`, `forward`)

1. Im SSE-Zweig statt `sse.CopyObserve(w, flush, resp.Body, observe)`:
   `redact.CopyRehydrated(w, flush, resp.Body, req.Placeholders, observe)`.
2. Im JSON-Zweig bei `len(req.Placeholders) > 0`: Body bis zur Grenze lesen,
   `redact.RehydrateBody`, `Content-Length` vor `WriteHeader` entfernen bzw. neu setzen.
3. `cmd/tollgate/main.go`: Startwarnung „clients see placeholders“ entfernen.

## Tradeoffs

- Byte-Splicing statt Neu-Serialisieren: hält unbekannte Felder und Reihenfolge stabil.
- Ein zusätzliches Event am Blockende statt Anhängen an das Stop-Event: Das Stop-Event hat
  kein Textfeld; ein Delta ist die einzige Form, die Clients verstehen.
- Platzhalter in Objekt-Schlüsseln von `partial_json` werden im Stream mit ersetzt (kein
  Kontext), im Nicht-Streaming-Fall nur String-Werte. Das Modell erzeugt Schlüssel aus dem
  Tool-Schema, der Fall ist praktisch ausgeschlossen.

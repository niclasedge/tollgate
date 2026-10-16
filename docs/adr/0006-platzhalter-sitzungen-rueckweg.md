# ADR-0006: Platzhalter, Sitzungen und Rückweg

- **Status:** Angenommen (Freigabe Niclas, 2026-10-04, PR #26)
- **Datum:** 2026-10-04
- **Issue:** #1

## Kontext

Der Auftrag verlangt nummerierte, deterministische Platzhalter (`<redacted:email_1>`). Die
Zuordnung soll nur im RAM liegen, mit TTL pro Sitzung. Beim Streamen werden die Platzhalter
zurückübersetzt, auch in `input_json_delta`, mit Puffer bis zum vollständigen Platzhalter.
Neue Regeln gelten erst für neue Sitzungen.

Beim Durchdenken zeigen sich vier Punkte, die der Auftrag offenlässt:

1. **Rückübersetzte Werte kehren zurück.** Das Modell schreibt `<redacted:email_1>` in einen
   Text oder `tool_use`. tollgate übersetzt das für Claude Code zurück. Im nächsten Turn
   schickt Claude Code den Assistant-Block mit dem **Klartext** als Verlauf. Wird nur
   User-Text und `tool_result` redigiert, geht der Klartext upstream, und der Präfix weicht
   vom ursprünglichen Modell-Output ab.
2. **Thinking-Signaturen.** `thinking`-Blöcke tragen eine Signatur über den Inhalt, den das
   Modell erzeugt hat, also mit Platzhaltern. Übersetzt tollgate `thinking_delta` zurück,
   schickt Claude Code im nächsten Turn veränderten Thinking-Text mit alter Signatur zurück.
   Die API lehnt das ab. Der Auftrag verbietet ohnehin, Thinking-Blöcke zu verändern.
3. **Subagenten.** Claude-Code-Subagenten (Task-Tool) laufen unter derselben Sitzungs-ID, aber
   mit eigenem Verlauf. Ein sitzungsweiter Zähler würde `email_1` in zwei Verläufen
   unterschiedlich belegen oder Lücken erzeugen. Beides wäre nicht deterministisch bezogen
   auf den einzelnen Request.
4. **`count_tokens`** enthält dieselben `messages` wie `/v1/messages`. Unverändert
   durchgereicht ginge Klartext upstream, und die Zählung passte nicht zur tatsächlich
   gesendeten Fassung.

## Entscheidung

### Platzhalter-Grammatik

```
<redacted:TYP_N>
TYP = [a-z][a-z0-9_]{0,31}     (Platzhalter-Typ, z. B. email, api_key, customer_id)
N   = [1-9][0-9]{0,5}          (laufende Nummer pro TYP)
```

Daraus folgen:
- Maximale Länge von 50 Bytes. Das begrenzt den Puffer im Rückweg.
- Erkennungs-Regex `<redacted:([a-z][a-z0-9_]{0,31})_([1-9][0-9]{0,5})>`.

### Nummerierung pro Anfrage, aus der Anfrage abgeleitet

1. Die Redact-Stufe sammelt alle Treffer im gesamten Request-Body in Dokumentreihenfolge
   (Byte-Offset). Dabei berücksichtigt sie nur die erlaubten Blöcke laut Tabelle unten.
2. Pro `TYP` bekommt jeder **verschiedene** Wert beim ersten Auftreten die nächste freie
   Nummer. Gleicher Wert ergibt gleiche Nummer, innerhalb der Anfrage.
3. **Reservierte Nummern.** Steht im Request bereits Text in Platzhalterform (z. B. vom
   Nutzer getippt oder aus einem älteren Log kopiert), werden dessen Nummern übersprungen.
   Ein solcher Text wird selbst nie zurückübersetzt. So kann ein eingeschleuster Platzhalter
   keinen fremden Wert freilegen.
4. **Zuordnung nur im RAM.** Die Zuordnung `TYP_N → Wert` existiert nur im RAM des Requests
   und wird nach dem Ende des Antwort-Streams verworfen. Sie wird nie geloggt, nie
   gespeichert und nie ans Frontend geschickt.

Warum das deterministisch und cachefest ist:
- Ein Claude-Code-Verlauf wächst nur nach hinten. Die Reihenfolge des ersten Auftretens ist
  für den unveränderten Präfix in jedem Turn gleich.
- Damit erzeugt der Präfix in jedem Turn dieselben Platzhalter, bytegenau und ohne
  gespeicherten Zustand.
- Neue Werte im neuen Turn bekommen höhere Nummern und ändern den Präfix nicht.

### Sitzung und Konfigurationsversion

- **Sitzungsschlüssel** = `SHA-256(client_id ‖ session_id)`. `session_id` stammt aus dem
  Header `X-Claude-Code-Session-Id`, sonst aus dem Session-Anteil von `metadata.user_id`.
  Fehlt beides, gilt der Schlüssel „Client ohne Sitzung“ und damit die jeweils aktive
  Version. Welche Quelle Claude Code 2.1.x tatsächlich liefert, verifiziert #5 an einer
  aufgezeichneten Anfrage.
- **Pinning:** Beim ersten Request einer Sitzung speichert tollgate in der Tabelle `sessions`
  die gerade aktive Konfigurationsversion. Alle weiteren Requests der Sitzung nutzen diese
  Version, für Redact und Compress gemeinsam (ADR-0008).
- Die Tabelle enthält nur Schlüssel-Hash, Client, Version und Zeitpunkte, keine Geheimnisse.
  Ein Neustart ändert also nichts am Pinning.
- Sitzungen ohne Aktivität seit 7 Tagen werden aufgeräumt. Ein späterer Request startet sie
  neu mit der dann aktiven Version.

### Welche Blöcke redigiert werden

| Block | Richtung | Behandlung |
|---|---|---|
| `messages[].content` als String (User) | Hinweg | redigiert |
| User-`text` | Hinweg | redigiert |
| `tool_result.content` (String oder Array von `text`-Blöcken) | Hinweg | redigiert |
| Assistant-`text` | Hinweg | **redigiert** (Klartext aus dem vorigen Rückweg) |
| `tool_use.input`, alle String-Werte rekursiv | Hinweg | **redigiert** (Klartext aus dem vorigen Rückweg) |
| `thinking`, `redacted_thinking`, `signature` | Hinweg | nie verändert |
| `system`, `tools`, `metadata` | Hinweg | nie verändert |
| `image`, `document` und unbekannte Blocktypen | Hinweg | nie verändert |
| `text_delta` | Rückweg | zurückübersetzt |
| `input_json_delta` | Rückweg | zurückübersetzt, Wert JSON-escaped (ADR-0004) |
| `thinking_delta`, `signature_delta` | Rückweg | **nicht** zurückübersetzt |
| alle übrigen Events (`message_start`, `message_delta`, `ping`, `error`, unbekannte) | Rückweg | unverändert |
| Nicht-Streaming-Antwort (`stream: false`) | Rückweg | `text` und `tool_use.input` per Byte-Splicing zurückübersetzt, Thinking unverändert |

Unbekannte Platzhalter im Modell-Output, also ohne Eintrag in der Zuordnung des Requests,
bleiben unverändert stehen.

### Rückweg im Stream

- **Ein Puffer pro Content-Block** (`index` aus `content_block_start`), weil Deltas
  verschiedener Blöcke nicht verschränkt werden dürfen.
- **Pro Delta:**
  1. Puffer plus neuer Text ergibt den Arbeitstext.
  2. Vollständige Platzhalter werden ersetzt.
  3. Endet der Arbeitstext mit einem Präfix, der noch zu einem Platzhalter werden kann (`<`,
     `<r`, …, `<redacted:email_`), bleibt dieser Rest im Puffer, höchstens 49 Bytes.
  4. Der Rest wird als Delta mit unveränderter Event-Struktur ausgegeben.
- **Ende eines Blocks:** Bei `content_block_stop` wird der Puffer ungeprüft ausgegeben.
- **`input_json_delta`:** `partial_json` ist ein Ausschnitt eines JSON-Dokuments. Ein
  Platzhalter liegt immer innerhalb eines JSON-Strings, denn das Modell hat ihn als Text
  erzeugt. Der Ersatzwert wird deshalb mit der Escape-Funktion aus ADR-0004 eingesetzt.
- **Events** werden einzeln geparst und neu geschrieben. Nur das Feld `delta.text` bzw.
  `delta.partial_json` ändert sich, und zwar per Byte-Splicing. Event-Grenzen, `event:`-Zeilen
  und `ping`s bleiben erhalten.
- **Leere Deltas:** Ein Delta, das durch das Puffern leer würde, wird trotzdem
  weitergeleitet, mit leerem Text. Die Zahl der Events bleibt damit gleich, und Clients sehen
  keine Lücken.

### `count_tokens` und andere Endpunkte

- `POST /v1/messages/count_tokens` läuft durch Eingang, Redact und Compress mit derselben
  gepinnten Version, dann durch Forward. Es gibt keinen Rückweg, die Antwort enthält nur
  Zahlen.
- Alle anderen Endpunkte (`/v1/models` u. a.) werden unverändert durchgereicht (ADR-0001).

### Upstream-Header für transformierte Requests

- `Accept-Encoding: identity` für `/v1/messages`, damit der Rückweg unkomprimiertes SSE
  bekommt. Die Strecke geht über localhost, das kostet nichts.
- `Content-Length` wird nach dem Splicing neu gesetzt.
- Alles Weitere siehe ADR-0001, Punkt 5.

## Alternativen

- **Zuordnung pro Sitzung mit TTL** (Wortlaut des Auftrags): verworfen.
  - Kollidiert bei Subagenten mit gleicher Sitzungs-ID.
  - Geht bei einem Neustart verloren, danach würden Platzhalter neu nummeriert und der
    Cache bräche.
  - Hält Geheimnisse länger im RAM als nötig.
  - Die Ableitung pro Anfrage ist strenger (nur RAM, nur für die Dauer des Requests) und
    zustandslos.
- **Nur User-Text und `tool_result` redigieren** (Wortlaut des Auftrags): verworfen, siehe
  Kontext, Punkt 1. Ohne erneutes Redigieren der Assistant-Blöcke gingen zurückübersetzte
  Geheimnisse im Klartext upstream.
- **HMAC-Platzhalter** (`<redacted:email_3f9a…>`): stabil ohne Zustand, aber nicht nummeriert,
  schlechter lesbar und länger. Das Modell kopiert lange Hashes außerdem fehleranfälliger.
- **Thinking ebenfalls zurückübersetzen:** verworfen. Das bricht die Signaturprüfung im
  nächsten Turn.

## Konsequenzen

- Im Thinking sieht der Nutzer Platzhalter statt Klartext. Das ist gewollt und im Frontend
  sowie in `docs/CLIENT_SETUP.md` erklärt.
- Komprimiert Claude Code den Verlauf (Compaction), verschieben sich die Nummern. Das ist
  unkritisch: Der Präfix ändert sich dann ohnehin, und innerhalb einer Anfrage bleibt die
  Zuordnung konsistent.
- **#13** setzt Nummerierung, Blocktabelle und Sitzungs-Pinning um.
- **#14** setzt den Rückweg um, inklusive Tests mit Platzhaltern über Chunk-Grenzen und in
  `input_json_delta`.
- **#17** belegt mit dem Cache-Test, dass mehrere Turns identische Präfixe erzeugen, auch mit
  Rückübersetzung und erneuter Redaktion sowie nach einer Konfigurationsänderung mitten in
  der Sitzung.
- **Offenes Risiko, Spike in #13:**
  - Claude Code bildet einen `cc_version`-Fingerprint aus einzelnen Zeichen der ersten
    User-Nachricht und legt ihn im Billing-Block in `system[0]` ab.
  - Ändert Redact genau diese Zeichen, passt der Fingerprint nicht mehr. CLIProxyAPI erzeugt
    den Block nur neu, wenn er fehlt.
  - Option, falls Anthropic den Fingerprint prüft: Den Billing-Block entfernen, wenn die erste
    User-Nachricht verändert wurde, damit CLIProxyAPI ihn über den redigierten Inhalt neu
    erzeugt. Das wäre deterministisch.
  - Diese Option würde die Regel „system nie verändern“ für genau diesen Block aufheben und
    braucht dann ein eigenes ADR.

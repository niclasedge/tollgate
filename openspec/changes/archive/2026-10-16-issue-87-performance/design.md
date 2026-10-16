# Design

Grundlage: AGENTS.md (harte Regeln, Nachvollziehbarkeit), ADR-0006 (Rückweg), ADR-0009
(kein Klartext), die Specs `store`, `proxy`, `meter`, `dashboard-api`, `frontend-views` und
`frontend-shell`. Alle Zeiten werden mit der monotonen Uhr (`time.Since`) gemessen und in
Mikrosekunden gespeichert, wie `traces.duration_us` heute.

## 1. Messwerte je Anfrage

### Bezugspunkt

Alle Zeiten einer Anfrage beziehen sich auf denselben Startpunkt wie die heutige
Gesamtdauer: `start` am Anfang von `serveProxy`. TTFT und Gesamtdauer enthalten damit
dieselben Anteile (Schlüsselprüfung, Body-Read, Pipeline), und `ttft_us <= duration_us` gilt
immer.

### TTFT (Zeit bis zum ersten Token)

| Antwort | TTFT endet, wenn … |
|---|---|
| SSE (`text/event-stream`) | das erste Event mit `type: content_block_delta` (gleich welcher Delta-Typ, auch `thinking_delta`) an den Client geschrieben und geflusht ist |
| JSON (`stream: false`) | die Antwort-Header des Upstreams da sind (`client.Do` kehrt zurück) |
| Status ≥ 400, Abbruch, kein `content_block_delta`, `count_tokens`, Passthrough | kein Wert (`NULL`) |

Begründung: `message_start` kommt sofort nach Annahme, noch vor der Generierung; das erste
Byte würde TTFT systematisch schönen. Das erste `content_block_delta` ist der Moment, in dem
Claude Code sichtbar etwas zeigt, deshalb zählt es nach dem Rückweg (nach Rehydrierung,
also beim Schreiben). Bei JSON-Antworten kommen die Header erst mit der fertigen Antwort;
TTFT ist dort fast gleich der Gesamtdauer und wird ehrlich so gezeigt.

Gemessen wird in `server.forward`: Der vorhandene `observe`-Callback von
`sse.CopyObserve` bekommt jedes Event vor dem Schreiben; ein kleiner Zähler merkt sich den
ersten `content_block_delta` und nimmt die Zeit direkt nach dem Flush dieses Events. Dafür
bekommt `sse.CopyObserve` keinen neuen Parameter; `server` wickelt `flush` so, dass nach
dem markierten Event einmal die Zeit genommen wird.

### Ausgabe-Tokens pro Sekunde

`output_tps = usage_output / ((duration_us − ttft_us) / 10⁶)`, gerundet auf eine
Nachkommastelle. Nur für SSE-Antworten mit `usage_output ≥ 1` und `duration_us > ttft_us`,
sonst `null`. Wird beim Lesen berechnet, nicht gespeichert (abgeleiteter Wert).

### tollgate-Eigenanteil

`tollgate_us = Σ trace_stages.duration_us + return_us` (fehlender `return_us` zählt als 0).
Das sind genau die Zeiten, in denen tollgate selbst rechnet: alle Stufen vor dem Forward
und die Rückübersetzung. Der Rest der Gesamtdauer heißt in der Oberfläche
„Upstream und Netz“; dazu gehören auch Body-Read vom Client und Gegendruck beim Schreiben.
Wird beim Lesen berechnet.

### Rückweg-Messwerte (aus #14)

- `return_us`: Summe der Zeit, die der Rückweg in der Rückübersetzung verbringt (je
  Event gemessen um den Rehydrate-Aufruf, ohne Warten auf den Upstream; bei JSON um das
  Byte-Splicing). `NULL`, wenn kein Rückweg lief (Redact aus, kein Platzhalter-Kontext,
  andere Endpunkte).
- `return_replacements`: Zahl der zurückübersetzten Platzhalter.

#14 stellt die Messpunkte bereit (Zähler im Rückweg-Typ). Fehlen sie bei der Umsetzung
noch, bleiben die Spalten `NULL` und die Redactor-Kosten zeigen nur die Hinweg-Stufe.

### Stufen-Bytes

`server.recordTrace` übernimmt heute `BytesIn/BytesOut` der Stufen nicht. Für die
Bezugsgröße „Bytes“ der Redactor-Kosten wird das ergänzt (Länge der Bodies vor und nach der
Stufe, aus dem Runner).

## 2. Store-Erweiterung

Neue Migration `00NN_trace_performance.sql` (nächste freie Nummer bei der Umsetzung, Stand
heute `0006`):

```sql
ALTER TABLE traces ADD COLUMN ttft_us INTEGER;             -- NULL = unbekannt
ALTER TABLE traces ADD COLUMN return_us INTEGER;           -- NULL = kein Rückweg
ALTER TABLE traces ADD COLUMN return_replacements INTEGER; -- NULL = kein Rückweg

-- Vergleichsabfrage: gleiches Modell, Zeitfenster vor der Anfrage.
CREATE INDEX traces_model_started_idx ON traces (model, started_at DESC, id DESC);

-- Urteil je Anfrage und Messgröße, einmal beim Abschluss berechnet.
CREATE TABLE trace_perf (
    trace_id   TEXT NOT NULL REFERENCES traces (id) ON DELETE CASCADE,
    metric     TEXT NOT NULL CHECK (metric IN ('ttft', 'duration')),
    class      TEXT NOT NULL,      -- z. B. 'i2' oder 'i2o1', siehe Abschnitt 3
    samples    INTEGER NOT NULL CHECK (samples >= 0),
    median_us  INTEGER,            -- NULL bei 'unbekannt'
    p90_us     INTEGER,
    verdict    TEXT NOT NULL CHECK (verdict IN ('normal', 'langsam', 'unbekannt')),
    PRIMARY KEY (trace_id, metric)
) STRICT;

-- Reservierter interner Client für den Speedtest (Abschnitt 6).
INSERT INTO clients (id, name, key_hash, key_hint, status, created_at)
VALUES ('speedtest', 'tollgate:speedtest', zeroblob(32), '', 'gesperrt', 0);
```

- Nur Zahlen und feste Bezeichner; `class` und `verdict` stammen aus festen Listen und
  werden validiert. Keine Inhalte, kein Text der Anfrage.
- `store.TraceResult` bekommt `TTFTMicros *int64`, `ReturnMicros *int64`,
  `ReturnReplacements *int64`; `store.Trace` liest sie mit. Negative Werte → `ErrInvalid`.
- `store.PutPerf(ctx, traceID, []Perf)` schreibt das Urteil, `store.PerfSamples(ctx,
  query)` liefert die Vergleichswerte (Abschnitt 3).
- `key_hash` aus 32 Nullbytes hat kein bekanntes Urbild, und der Status `gesperrt` lässt
  jede Schlüsselprüfung scheitern. `CreateClient`, `RotateClientKey` und die Statusänderung
  lehnen die ID `speedtest` und Namen mit Präfix `tollgate:` ab; die Clients-Liste der API
  blendet den Client aus.

## 3. Erwartungswert-Modell: Perzentile je Größenklasse (gewählt)

### Größenklassen

`input_total = usage_input + usage_cache_read + usage_cache_creation`.

| Eingabe-Klasse | `input_total` | Ausgabe-Klasse | `usage_output` |
|---|---|---|---|
| `i0` | < 8 000 | `o0` | < 256 |
| `i1` | 8 000 – 31 999 | `o1` | 256 – 1 023 |
| `i2` | 32 000 – 127 999 | `o2` | 1 024 – 4 095 |
| `i3` | ≥ 128 000 | `o3` | ≥ 4 096 |

Faktor 4 je Klasse: fein genug, dass der Median aussagekräftig ist, grob genug, dass sich
schnell 20 Vergleichswerte sammeln.

- **TTFT** hängt vor allem an der Eingabe: Klasse = Modell × Eingabe-Klasse (`i2`).
- **Gesamtdauer** hängt an Eingabe und Ausgabe: Klasse = Modell × Eingabe × Ausgabe
  (`i2o1`). Hat diese Klasse weniger als 20 Vergleichswerte, wird **einmal** auf Modell ×
  Ausgabe-Klasse (`o1`) zurückgefallen. Die gespeicherte `class` zeigt, welche galt.

### Datenbasis

Traces desselben Modells (exakter String, z. B. `claude-opus-4-1[1m]`) mit
`started_at` in `[t − 14 Tage, t)`, `t` = Start der beurteilten Anfrage; nur
`endpoint = 'messages'`, Status 200–299, `error_kind = ''`, Usage vorhanden, Messgröße nicht
`NULL`, nicht der Speedtest-Client. Von diesen die neuesten 500 (`started_at DESC, id
DESC`). 14 Tage decken Wochentags-Schwankungen ab und reagieren trotzdem auf
Modell-Updates; 500 begrenzt die Kosten der Abfrage.

### Statistik

- Perzentile nach der **Nearest-Rank-Methode** auf den sortierten Ganzzahlen:
  `p(q) = x[⌈q · n⌉ − 1]`, Median = `p(0,5)`. Keine Interpolation, keine Gleitkomma-Mittel:
  gleiche Werte ergeben Byte für Byte dasselbe Ergebnis.
- Mindestens **20** Vergleichswerte, sonst `unbekannt` („noch zu wenig Vergleichswerte“).

### Schwelle

Eine Messgröße ist `langsam`, wenn alle drei Bedingungen gelten:

1. Wert > p90 der Klasse,
2. Wert ≥ 1,5 × Median,
3. Wert − Median ≥ 1 s.

Begründung: p90 allein markiert per Konstruktion jede zehnte Anfrage rot und ist damit
Rauschen. Der Faktor 1,5 verlangt einen spürbaren Abstand, die absolute 1 s verhindert Rot
bei kleinen Werten (0,4 s statt 0,25 s). Eine Anfrage ist rot, wenn TTFT **oder**
Gesamtdauer `langsam` ist.

### Zeitpunkt und Determinismus

Das Urteil wird in `finishTrace` direkt nach `FinishTrace` berechnet und in `trace_perf`
gespeichert, bevor das Live-Event rausgeht. Die Antwort an den Client ist zu diesem
Zeitpunkt schon vollständig. Die Bewertungsfunktion `perf.Judge(samples, value)` ist rein;
ein gespeichertes Urteil ändert sich nie, auch wenn später neue Daten kommen. Ein Fehler
der Abfrage wird geloggt und ergibt keine Zeile in `trace_perf` (die API zeigt dann kein
Urteil); er blockiert nie den Proxy.

### Erklärung

Die API liefert Zahlen, das Frontend formt den Text:

- langsam: „Gesamtdauer: erwartet ~4,2 s, gebraucht 9,8 s (p90 6,1 s, 37 Vergleichswerte)“
- normal: „im üblichen Rahmen (Median 4,2 s)“
- unbekannt: „noch zu wenig Vergleichswerte (7 von 20)“

### Verworfene Alternativen

- **Lineare Regression `Dauer ≈ a + b · Output`** (robust per Theil-Sen): nutzt alle Daten
  eines Modells und braucht keine Klassen, aber der Erwartungswert ist schwerer zu erklären,
  die Schwelle braucht eine Residuen-Statistik, und Theil-Sen ist O(n²). Lohnt erst, wenn
  die Klassen nachweislich zu dünn sind.
- **Gleitender Durchschnitt ohne Klassen:** ignoriert die Größe der Anfrage; eine lange
  Antwort wäre immer „langsam“.
- **Urteil beim Lesen berechnen:** hält keine zusätzlichen Daten, aber jede Seite der
  Live-Liste bräuchte viele Abfragen, und das Urteil einer alten Anfrage könnte kippen.

## 4. Performance-API

### `GET /api/performance?days=N`

`N` von 1 bis 90, Default 7; 1 = heutiger UTC-Tag wie bei `/api/costs`. Die Oberfläche
bietet 1, 7 und 30. Basis: abgeschlossene `messages`-Traces mit Status 2xx im Zeitraum,
ohne Speedtest-Client.

```go
type Dist struct { Samples int64; Min, Median, P90, Max int64 }       // µs, Nearest-Rank
type TPSDist struct { Samples int64; Min, Median, P90, Max float64 }  // Tokens/s, 0,1 genau
type DayPerf struct { Day string; Requests, Slow int64; TTFTMedian, DurationMedian *int64 }
type ModelPerf struct {
    Model string; Requests, Slow, Judged int64   // Judged: Urteil normal oder langsam
    TTFT, Tollgate *Dist; Duration Dist; OutputTPS *TPSDist
    Days []DayPerf
}
type RedactorCost struct {
    Requests int64
    Stage, Return *Dist       // µs der Stufe redact bzw. des Rückwegs
    ShareMedianPermille int64 // Median von (Stufe + Rückweg) / Gesamtdauer, in ‰
    BytesInMedian, MatchesMedian, MatchesTotal, ReturnReplacementsTotal int64
    TokensAddedTotal, TokensAddedMedian int64 // tokens_out − tokens_in der Stufe (Schätzung)
}
type SpeedtestResult struct {
    Model string; Redact bool; TraceID, StartedAt string; Status int
    TTFT, Duration *int64; OutputTPS *float64; Error string
    UsualTTFTMedian, UsualDurationMedian *int64 // Median des Modells im Zeitraum
}
type PerfReport struct {
    From, To string; Models []ModelPerf; Redactor *RedactorCost; Speedtests []SpeedtestResult
}
```

JSON-Felder in snake_case mit Einheit im Namen (`median_us`, `ttft_median_us`); die
TypeScript-Typen entstehen mit `tygo`. Modelle sortiert nach `requests` absteigend, dann
Name; Tage aufsteigend. `slow_share` = `slow / judged` berechnet das Frontend.
`speedtests`: je Modell und Variante der neueste Lauf.

### `TraceSummary` (Live, Startseite, Events)

Neue Felder: `ttft_ms` (`number | null`), `output_tps` (`number | null`),
`tollgate_ms` (`number | null`) und `perf`:

```go
type PerfMetric struct { Verdict string; Class string; Samples int64; MedianMs, P90Ms *float64; ValueMs *float64 }
type TracePerf struct { Slow bool; TTFT, Duration *PerfMetric }   // perf: null ohne Urteil
```

Das `trace`-Event nach dem Abschluss trägt das Urteil bereits, weil es vor dem Event
berechnet wird.

## 5. Oberfläche

- **Sidebar:** neuer Eintrag „Performance“ in der Gruppe Beobachten nach „Kosten“,
  Route `performance`, Kürzel `8` (die vorhandenen Kürzel bleiben, damit sich nichts
  verschiebt).
- **Performance-Ansicht:** Zeitraum-Umschalter (Heute, 7 Tage, 30 Tage). Je Modell eine
  Zeile mit Anzahl, Anteil langsam, TTFT und Gesamtdauer als min / median / p90 / max,
  Ausgabe-Tokens/s (median, p90); Aufklappen zeigt einen dezenten Verlauf je Tag (Median
  TTFT und Dauer als schmale Linie, Tage mit langsamen Anfragen als roter Punkt) und den
  tollgate-Eigenanteil. Darunter Karte „Redactor-Kosten“ und Karte „Speedtest“.
- **Redactor-Kosten:** Stufe Redact und Rückweg je median / p90 / max in ms, Anteil an der
  Gesamtdauer (Median), Bezugsgrößen Bytes und Treffer, zusätzliche Tokens (Summe, Median)
  mit dem Hinweis: „Die Platzhalter sind deterministisch. Der Redactor ändert den
  Prompt-Cache nicht; zusätzliche Tokens sind eine Schätzung (Bytes / 4).“
- **Roter Statuspunkt:** `StatusDot` bekommt den Zustand `slow` (Farbe `--c-error`), mit
  der Erklärung als `title`/`aria-label`. Vorrang: laufend > Fehler > langsam > ok. Live
  zeigt ihn in der Liste und im Inline-Detail mit dem Erklärtext und TTFT, Tokens/s,
  tollgate-Anteil; die Startseite in den letzten 10 Anfragen. Ohne Urteil bleibt der Punkt
  wie bisher, im Detail steht „noch zu wenig Vergleichswerte“.

## 6. Speedtest

### Ablauf

`POST /api/speedtest` mit `{"models": ["…"], "confirm": true}`.

- 1 bis 4 Modelle, jedes ein gültiger Bezeichner (wie `traces.model`). Ohne
  `confirm: true` antwortet tollgate 400 mit „Bestätigung fehlt: der Test verbraucht
  Abo-Kontingent“. Läuft schon ein Test, 409.
- Je Modell nacheinander: zuerst **ohne**, dann **mit** Redact-Stufe. Ist die Redact-Stufe
  ausgeschaltet (`redact.enabled: false`), wird „mit“ mit `error: "Redact-Stufe aus"`
  gemeldet statt gelaufen.
- Zeitlimit je Anfrage 60 s. Die Antwort enthält die `SpeedtestResult`-Liste.

### Feste Testanfrage

```json
{"model":"<MODELL>","max_tokens":400,"stream":true,
 "messages":[{"role":"user","content":"Antworte nur mit den Zahlen 1 bis 100, durch Kommas getrennt. Kontakt: speedtest@tollgate.test"}]}
```

Kein Geheimnis. Die erfundene Adresse gibt der Redact-Stufe und dem Rückweg einen Treffer,
damit „mit Redact“ beide Wege wirklich durchläuft (ein Test sichert, dass sie erkannt wird;
sonst wird eine andere erfundene Adresse gewählt). Rund 200 Ausgabe-Tokens reichen für eine
belastbare Tokens/s-Zahl. Header: `anthropic-version: 2023-06-01`,
`User-Agent: tollgate-speedtest`. CLIProxyAPI kann die Anfrage tarnen (Cloaking); das gilt
für beide Varianten gleich und verfälscht den Vergleich nicht.

### Interner Aufruf über dieselbe Pipeline

- `server.Server` bekommt `Probe(ctx, ProbeInput) (traceID string, err error)`. Probe baut
  eine `http.Request` für `POST /v1/messages` und ruft denselben Code wie `serveProxy`
  (Account-Gate, Pipeline, Trace, Forward, Meter, TTFT, Urteil) mit einem
  `ResponseWriter`, der die Antwort verwirft. Es gibt keinen Netzwerk-Umweg und keinen
  zweiten Code-Pfad zum Upstream.
- Identität: `pipeline.WithInternalClient(ctx, InternalClient{ID: "speedtest",
  SkipRedact: bool})`. `Intake.Authenticate` nimmt diese Identität aus dem Kontext statt
  eines Schlüssels. Der Kontextschlüssel ist ein nicht exportierter Typ; über HTTP ist er
  nicht erreichbar. Der Runner überspringt die Stufe `redact` nur bei dieser Identität;
  andere Stufen lassen sich nicht abschalten.
- Ergebnis: Probe gibt die Trace-ID zurück, der Handler liest den Trace (TTFT, Dauer,
  Usage, Status). Die Variante ergibt sich aus dem Vorhandensein der Stufe `redact`.
- Speedtest-Traces sind normale Traces des Clients `tollgate:speedtest`: sichtbar in Live
  und Kosten (sie kosten echtes Kontingent), aber nie Vergleichswert und nicht in der
  Modellstatistik.

### `just speedtest`

`just speedtest models="claude-…"` ruft `deploy/setup.sh speedtest`. Das Skript fragt
„Der Test verbraucht Abo-Kontingent. Fortfahren? [j/N]“ (mit `--yes` ohne Frage), schickt
das Admin-Token wie `cmd_client` über stdin an `curl` und gibt je Modell und Variante eine
Zeile mit TTFT, Dauer, Tokens/s und dem üblichen Median aus.

## 7. Abhängigkeiten

| Teil | hängt ab von |
|---|---|
| TTFT, Tokens/s, Store-Spalten, Urteil, Performance-Ansicht ohne Redactor-Karte, roter Punkt | nichts Offenem |
| Redactor-Kosten Hinweg (Stufe `redact`, Bytes, Treffer, Tokens) | #13 |
| Rückweg-Dauer und Rückübersetzungen, TTFT nach Rehydrierung, tollgate-Anteil vollständig | #14 |
| Speedtest „mit Redact“ inkl. Rückweg | #13, #14 |
| Platz in Sidebar und Startseite | #76 (gemergt) |

Die Umsetzung beginnt erst, wenn die laufenden Issues fertig sind (Wunsch Niclas); dann sind
#13 und #14 voraussichtlich geschlossen, und es gibt keinen Teilausbau.

## 8. Tests

- `internal/perf`: Tabellen für Nearest-Rank (n = 1, 2, 20, 21, gleiche Werte),
  Klassengrenzen (7 999/8 000, 255/256 …), Rückfall der Dauer-Klasse, Schwelle (jede der
  drei Bedingungen einzeln verletzt), `unbekannt` bei 19 Werten, Determinismus
  (permutierte Eingabe → gleiches Ergebnis), Tokens/s-Grenzfälle.
- `internal/store`: Migration auf bestehender DB, neue Spalten lesen/schreiben, Wertebereiche,
  `trace_perf` Kaskade, reservierter Client nicht anlegbar/rotierbar und nicht über einen
  Schlüssel authentifizierbar, Vergleichsabfrage (Fenster, Filter, Limit 500, Reihenfolge).
- `internal/server`: `httptest`-SSE-Upstream mit künstlicher Verzögerung vor dem ersten
  `content_block_delta` → `ttft_us` im Bereich; JSON-Upstream mit verzögerten Headern;
  Fehlerstatus → `NULL`; künstlich verzögerte Anfrage nach 20 schnellen wird `langsam`;
  Probe ohne/mit Redact (Stufe fehlt/vorhanden), interner Kontext nicht über Header
  erreichbar; Leck-Test: Speedtest-Text nicht in DB und Log.
- `internal/dashboard`: Performance-Bericht (Verteilung, Sortierung, Zeitraum, Speedtest
  ausgeschlossen), Speedtest ohne Bestätigung 400, parallel 409, Admin-Token Pflicht.
- Frontend (Vitest): roter Punkt und Erklärtext bei `perf.slow`, „noch zu wenig
  Vergleichswerte“, Performance-Tabelle mit min/median/p90/max, Speedtest mit Mock und
  Bestätigungsdialog, Sidebar-Eintrag.

## Festgelegte Detailfragen (2026-10-04)

Die Detailfragen wurden nach der Empfehlung entschieden; Niclas hatte freigegeben, Empfehlungen direkt umzusetzen.

1. Speedtest-Anfragen erscheinen in Live und Kosten, weil sie echtes Kontingent verbrauchen. In die Modellstatistik und in die Vergleichswerte gehen sie nicht ein.
2. Der interne Client ist eine reservierte Zeile, angelegt per Migration: `id = speedtest`, Name `tollgate:speedtest`, gesperrt. Die Client-API blendet sie aus.
3. Performance bekommt die neue Taste `8`. Die übrigen Kürzel bleiben, wie sie sind.
4. Die Testanfrage enthält die erfundene Adresse `speedtest@tollgate.test`, damit „mit Redact“ Hinweg und Rückweg tatsächlich durchläuft.

# Design

Grundlage: Change `issue-87-performance` (Messwerte, Größenklassen `i0`–`i3`/`o0`–`o3`,
Nearest-Rank, Datenbasis 14 Tage / 500 Werte, Performance-API), AGENTS.md (Determinismus,
nur redigierte Daten). Alles hier ist eine Erweiterung; #87 bleibt unverändert.

## 1. Tokens/s als Ganzzahl

`output_tps` aus #87 ist ein abgeleiteter Wert mit einer Nachkommastelle. Für Vergleich und
Speicherung rechnet dieser Change in **Milli-Tokens pro Sekunde** (Ganzzahl):

`tps_milli = usage_output · 10⁹ / (duration_us − ttft_us)` (Ganzzahl-Division)

Gleiche Bedingungen wie `output_tps` (SSE, `usage_output ≥ 1`, `duration_us > ttft_us`).
Die Oberfläche zeigt `tps_milli / 1000` mit einer Nachkommastelle; damit stimmen Live und
#87 überein.

## 2. Urteil Tokens/s

### Klasse und Datenbasis

- Klasse: Modell × Eingabe-Klasse (`i0`–`i3` wie #87), weil lange Kontexte die
  Generierung messbar bremsen können. Weniger als 20 Vergleichswerte → **einmal** Rückfall
  auf das Modell ohne Klasse (`class = '*'`).
- Datenbasis wie #87 (gleiches Modell, `[t − 14 Tage, t)`, `messages`, 2xx, kein Fehler,
  kein Speedtest-Client, neueste 500), zusätzlich: SSE, `ttft_us` vorhanden,
  `usage_output ≥ 64`.
- **Mindestausgabe 64 Tokens** für Vergleichswert und beurteilte Anfrage: Bei sehr kurzen
  Antworten dominiert der letzte Netzwerk-Flush, Tokens/s ist dann Rauschen.

### Schwelle

Nearest-Rank auf `tps_milli`: p10, Median, p90. Abweichung `d = (tps − median) / median`
in Promille (Ganzzahl).

| Urteil | Bedingung |
|---|---|
| `langsamer` | `tps < p10` **und** `d ≤ −200 ‰` |
| `schneller` | `tps > p90` **und** `d ≥ +200 ‰` |
| `üblich` | sonst |
| `unbekannt` | < 20 Vergleichswerte nach Rückfall oder `usage_output < 64` |

Begründung wie bei #87: Perzentil allein markiert jede zehnte Anfrage; die 20 % verlangen
einen spürbaren Abstand. „Durchschnitt“ ist bewusst der **Median**: ein einzelner
abgebrochener Stream verzerrt den Mittelwert stark, den Median nicht.

### Speicherung

Eigene Tabelle, damit `trace_perf` aus #87 unverändert bleibt und die Reihenfolge der
Migrationen keine Rolle spielt:

```sql
CREATE TABLE trace_tps (
    trace_id      TEXT PRIMARY KEY REFERENCES traces (id) ON DELETE CASCADE,
    class         TEXT NOT NULL,                 -- 'i0'..'i3' oder '*'
    samples       INTEGER NOT NULL CHECK (samples >= 0),
    tps_milli     INTEGER NOT NULL CHECK (tps_milli >= 0),
    p10_milli     INTEGER,                       -- NULL bei 'unbekannt'
    median_milli  INTEGER,
    p90_milli     INTEGER,
    deviation_permille INTEGER,                  -- NULL bei 'unbekannt'
    verdict       TEXT NOT NULL CHECK (verdict IN ('schneller', 'üblich', 'langsamer', 'unbekannt'))
) STRICT;
```

Berechnet in `finishTrace` direkt nach dem Urteil aus #87, vor dem Live-Event. Fehler werden
geloggt, ergeben keine Zeile und blockieren den Proxy nie. Ein gespeichertes Urteil ändert
sich nicht.

## 3. Einfluss von Eingabe und Cache

### Cache-Klassen

`cache_share = usage_cache_read · 1000 / input_total` (Promille, Ganzzahl;
`input_total` wie #87).

| Klasse | Cache-Anteil |
|---|---|
| `c0` | < 500 ‰ (kalt, meist neue Sitzung oder Cache abgelaufen) |
| `c1` | 500 – 899 ‰ |
| `c2` | ≥ 900 ‰ (warm, typischer Folge-Turn in Claude Code) |

### Matrix

Je Modell und Zeitraum (`days` wie `/api/performance`): für jede Kombination
Eingabe-Klasse × Cache-Klasse Anzahl, Median TTFT, Median `tps_milli`, Median Dauer.
Zellen mit weniger als **5** Werten liefern nur die Anzahl (Medianfelder `null`). Basis wie
die Modellstatistik aus #87; Tokens/s nur aus Anfragen mit `usage_output ≥ 64`.

### Kernaussagen

Rein aus der Matrix, deterministisch, je Modell höchstens zwei:

1. **Cache → TTFT:** je Eingabe-Klasse, in der `c0` und `c2` je ≥ 5 Werte haben:
   `Δ = (median_ttft(c2) − median_ttft(c0)) · 1000 / median_ttft(c0)`. Gemeldet wird die
   Eingabe-Klasse mit den meisten Werten (Gleichstand: kleinere Klasse).
   Text: „Warmer Cache (≥ 90 %) senkt TTFT bei 32k–128k Eingabe im Median um 41 %.“
2. **Eingabegröße → Tokens/s:** kleinste und größte Eingabe-Klasse mit je ≥ 5 Werten:
   `Δ = (median_tps(max) − median_tps(min)) · 1000 / median_tps(min)`.
   Text: „Bei ≥ 128k Eingabe generiert das Modell 18 % langsamer als bei < 8k.“

Die API liefert nur Zahlen (Klassen, Δ in Promille, Anzahl); den Satz formt das Frontend.
Unter der Matrix steht der Hinweis: „Zusammenhang aus den eigenen Daten, keine Ursache.“

### Verworfene Alternativen

- **Regression TTFT ≈ a + b · uncached_tokens:** genauer, aber schwer erklärbar und braucht
  Residuen-Statistik; erst sinnvoll, wenn die Matrix zu dünn bleibt.
- **Streudiagramm aller Anfragen:** zeigt mehr, ist aber auf dem iPhone unlesbar und
  braucht alle Rohwerte im Browser.

## 4. Trend

`DayPerf` aus #87 bekommt `TPSMedianMilli *int64` und `CacheSharePermilleMedian *int64`.
Die Performance-Ansicht zeigt im aufgeklappten Modell drei schmale Linien über die Tage:
Median TTFT, Median Tokens/s, Median Cache-Anteil. Damit wird sichtbar, ob ein Abfall der
Tokens/s mit sinkendem Cache-Anteil zusammenfällt oder modellseitig ist.

## 5. API

```go
type TPSJudgement struct {
    Verdict string            // schneller | üblich | langsamer | unbekannt
    Class string; Samples int64
    TPS float64               // tps_milli / 1000, eine Nachkommastelle
    P10, Median, P90 *float64
    DeviationPermille *int64
}
type InfluenceCell struct {
    Input, Cache string       // i0..i3, c0..c2
    Samples int64
    TTFTMedianUs, DurationMedianUs *int64
    TPSMedian *float64
}
type Insight struct {
    Kind string               // cache_ttft | input_tps
    Input, From, To string    // Klassen
    DeltaPermille int64; SamplesFrom, SamplesTo int64
}
type Influence struct { Cells []InfluenceCell; Insights []Insight }
```

- `TraceSummary.tps *TPSJudgement` (`null` ohne Zeile in `trace_tps`).
- `ModelPerf.Influence Influence`; Zellen sortiert nach Eingabe-, dann Cache-Klasse, alle
  12 Kombinationen immer vorhanden (leere mit `samples: 0`).
- JSON snake_case, TypeScript über `tygo`.

## 6. Oberfläche

- **Live-Liste:** neue Spalte „t/s“ zwischen Tokens und Gesparte Tokens: Zahl ohne
  Nachkommastelle, darunter klein die Abweichung („−23 %“). `langsamer` in der Fehlerfarbe,
  `schneller` im Akzentblau, `üblich` grau, `unbekannt` nur die Zahl. Tooltip:
  „48,2 t/s · üblich 62,0 (p10 51,3 – p90 70,1), 87 Vergleichswerte, Klasse 32k–128k“.
  Laufende Anfragen und Nicht-Streams: „–“. Auf schmalen Bildschirmen ausgeblendet.
- **Inline-Detail:** Zeile „Tokens/s“ mit dem Satz aus dem Tooltip.
- **Performance:** unter der Modelltabelle aufklappbar je Modell „Einfluss von Eingabe und
  Cache“: 4 × 3-Tabelle, Zellhintergrund in einem einzigen Blauton nach Median TTFT
  (heller = schneller), Zahl TTFT und t/s in der Zelle, Anzahl klein. Darüber die
  Kernaussagen als Sätze. Dazu die Trendlinien (Abschnitt 4).

## 7. Abhängigkeiten

| Teil | hängt ab von |
|---|---|
| alles | #87 (TTFT, `output_tps`, `PerfSamples`, `/api/performance`, Performance-Ansicht) |

## 8. Tests

- `internal/perf`: `TPSMilli`-Grenzfälle, Cache-Klassengrenzen (499/500, 899/900 ‰),
  Urteil (jede Bedingung einzeln verletzt, Rückfall auf `*`, 19 Werte, Ausgabe 63),
  Matrix (leere Zellen, Schwelle 5), Kernaussagen (Gleichstand, fehlende Klassen),
  Determinismus mit permutierter Eingabe.
- `internal/store`: Migration, `trace_tps` schreiben/lesen, Kaskade, Vergleichsabfrage
  mit Mindestausgabe.
- `internal/server`: 20 schnelle Streams, dann ein langsamer → Event mit `tps.verdict =
  langsamer`; Fehler beim Urteil blockiert nicht.
- `internal/dashboard`: `tps` in `TraceSummary`, `influence` mit 12 Zellen, Trendfelder.
- Frontend (Vitest): Spalte „t/s“ mit Abweichung und Farbe je Urteil, „–“ bei laufend,
  Matrix mit leeren Zellen, Kernaussagen-Text.

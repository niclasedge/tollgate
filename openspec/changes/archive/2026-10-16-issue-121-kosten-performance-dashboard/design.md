# Design

Grundlage: Kostenbericht (`dashboard-api`), Listenpreise (`meter`), Performance-Ansicht aus
#87/#116, AGENTS.md (Determinismus, nur Zahlen, Things-3-Stil).

## 1. Stunden-Buckets im Kostenbericht

`GET /api/costs?days=N` bekommt das Feld `hours`. Für `N ≤ 7` enthält es je **UTC-Stunde ×
Account × Modell** eine Zeile mit Anzahl, Usage und `cost_nano_usd`; für `N > 7` ist es eine leere
Liste (das Frontend nimmt dann die Tageszeilen).

- **Warum mit Modell:** Der Preis hängt am Modell. Bleibt das Modell in der Zeile, gilt dieselbe
  Regel wie bei den Tageszeilen: unbekannter Preis → `null`, ohne Teilsummen zu verstecken. Das
  Frontend summiert je Account wie bisher (`aggregate`, „≥“ bei Teilsummen).
- **Format:** `hour` = `YYYY-MM-DDTHH:00:00Z` aus `strftime` über `started_at` (UTC). Sortierung
  nach Stunde, Account, Modell. Nur abgeschlossene Traces, gleiche Datenbasis wie `CostByDay`.
- **Leere Stunden** liefert die API nicht; das Frontend füllt die Achse lückenlos mit 0. Damit
  bleibt die Antwort klein und eindeutig.

## 2. Cache-Ersparnis

`meter.CacheSavings(model, usage) = cache_read · (Preis Eingabe − Preis Cache-Lesen)` in
Nano-USD, ganzzahlig, `ok = false` bei unbekanntem Modell. `CostRow` trägt
`cache_savings_nano_usd` (`null` ohne Preis). Cache-Schreiben bleibt außen vor: es kostet mehr
als normale Eingabe, die Ersparnis entsteht erst beim Lesen.

## 3. Navigation

`routes` enthält `performance` nicht mehr; `parseHash` bildet `#/performance` auf `costs` ab,
und der Router ersetzt den Hash per `history.replaceState` durch `#/costs` (kein zusätzlicher
Verlaufseintrag). Label „Kosten & Performance“, Taste `3`. Die Taste `8` entfällt; die übrigen
Kürzel bleiben.

## 4. Seite „Kosten & Performance“

Ein Zeitraum (Heute = 1, 7, 30 Tage; Standard 7) steuert beide Abrufe (`/api/costs`,
`/api/performance`) parallel; Fehler werden je Teil angezeigt. `/api/accounts` liefert die Namen
(wie im Bereich Accounts); ohne Account-Verwaltung bleibt der Rückfall `Account <ID 12>`, ohne
Account „Nicht zugeordnet“.

Oben (Dashboard):

| Teil | Inhalt |
|---|---|
| Gesamtkosten | Summe groß, darunter Anfragen; Liste je Account: Kosten, Anteil an den bekannten Kosten, Tokens |
| Kostenkurve | gestapelte Flächen je Account, x = Stunde (≤ 7 Tage) bzw. Tag, Beschriftung in UTC |
| Kennzahlen | Tokens verarbeitet (alle vier Usage-Felder), Cache gelesen, Eingabe ungecacht (`input + cache_creation`), Ausgabe, Cache-Ersparnis in $, von tollgate gespart |
| Tokens/s | `TpsChart` aus #116, daneben je Modell TTFT-Median und Anteil „langsam“ |

Unten (Details): Aufschlüsselung mit Umschalter Account / Modell / Client / Tag (Spalten Anfragen,
Tokens, Cache-Quote, Cache-Ersparnis, Kosten, Anteil). In „Tag“ erscheinen nur Tage mit Zeilen,
neueste zuerst. Danach der Performance-Detailbereich (bisherige `Performance.svelte` ohne eigenen
Abruf und Zeitraum).

**Farben:** Account 1 im Akzentblau, weitere Accounts in Grautönen (`--c-text`, `--c-muted`,
`--c-idle`) mit abnehmender Deckkraft. Die Legende ist die Account-Liste der Gesamtkosten; jedes
Feld trägt dasselbe Farbmuster. Kein Orange, keine weiteren Farben.

**Bedienung:** Zeitraum und Umschalter sind Knopfgruppen mit `aria-pressed`, erreichbar mit Tab;
das Diagramm ist ein `role="img"` mit einer Textzusammenfassung je Account.

## Tests

- Go: `CostByHour` tabellengetrieben (Stundengrenze `HH:59:59.999` / `HH+1:00`, UTC, Tag-Wechsel,
  leere Stunden fehlen, unbeendete Traces fehlen); `CacheSavings` inkl. unbekanntem Modell; Handler:
  `hours` bei 1/7 Tagen, leer bei 8 Tagen, `cost_nano_usd`/`cache_savings_nano_usd` `null` ohne Preis.
- Vitest: Umleitung `#/performance`, Sidebar und Taste `3`; Gesamtsumme und Anteile je Account mit
  Namen aus `/api/accounts`; Kostenkurve mit einer Fläche je Account und lückenloser Stundenachse;
  Kennzahlen; Umschalter; Tage ohne Wert ausgeblendet; bestehende Performance-Tests auf der neuen Seite.

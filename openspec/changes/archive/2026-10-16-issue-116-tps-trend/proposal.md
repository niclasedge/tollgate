# Proposal

Issue: [#116 Live: Tokens/s mit Vergleich zum Üblichen, Einfluss von Eingabe- und Cache-Tokens (Trend)](https://github.com/niclasedge/tollgate/issues/116)

> **Baut auf #87 auf** (TTFT, `output_tps`, `trace_perf`, Performance-Ansicht). Umsetzung erst,
> wenn #87 gemergt ist. Der Change ist vorbereitet, aber noch nicht begonnen.

## Why

#87 macht Antwortzeiten messbar und markiert Ausreißer bei TTFT und Gesamtdauer. Zwei Fragen
bleiben offen:

1. In Live sieht man nicht auf einen Blick, wie schnell Claude gerade **generiert**
   (Tokens/s) und ob das dem Üblichen entspricht.
2. Ob große Eingaben oder ein hoher **Cache-Anteil** TTFT und Generierungsgeschwindigkeit
   beeinflussen, lässt sich nicht ablesen. Es fehlt eine Auswertung, die einen Trend zeigt.

## What Changes

- **Urteil Tokens/s:** Beim Abschluss einer Stream-Anfrage wird `output_tps` mit den
  Vergleichswerten desselben Modells und derselben Eingabe-Klasse verglichen und als
  `schneller`, `üblich`, `langsamer` oder `unbekannt` gespeichert (neue Tabelle `trace_tps`,
  mit p10/Median/p90 für eine erklärbare Aussage).
- **Live:** neue Spalte „t/s“ mit Abweichung zum Median (z. B. „48 · −23 %“), Tooltip mit
  Vergleichswerten; im Inline-Detail als Satz („48,2 t/s, üblich 62,0 t/s“).
- **Einfluss-Auswertung:** `GET /api/performance` liefert je Modell eine Matrix
  Eingabe-Klasse × Cache-Klasse (Median TTFT, Median Tokens/s, Median Dauer, Anzahl) und
  Kernaussagen (Effekt des Caches auf TTFT, Effekt der Eingabegröße auf Tokens/s).
- **Trend:** Der Tagesverlauf je Modell bekommt Median Tokens/s und Median Cache-Anteil.
- **Frontend:** Performance-Ansicht zeigt die Matrix als ruhige Heatmap-Tabelle, die
  Kernaussagen als Text und den Tagesverlauf als schmale Linien.

## Capabilities

### Modified Capabilities

- `performance` (aus #87): Urteil Tokens/s, Cache-Klassen, Einfluss-Matrix, Kernaussagen.
- `store`: Tabelle `trace_tps`, Vergleichsabfrage für Tokens/s.
- `dashboard-api`: Felder `tps` in `TraceSummary`, `influence` und Tagesfelder im
  Performance-Bericht.
- `frontend-views`: Spalte „t/s“ in Live, Einfluss-Matrix und Trend in Performance.

## Impact

Branch `issue-116/tps-trend`, gemeinsamer Workspace. Voraussichtlich berührte Dateien:

- Neu: `internal/store/migrations/00NN_trace_tps.sql` (nächste freie Nummer nach #87),
  `internal/perf/tps.go`, `internal/perf/influence.go`, `internal/store/tps.go`,
  `web/src/lib/components/TpsCell.svelte`, `web/src/lib/components/InfluenceMatrix.svelte`,
  zugehörige Tests.
- Eigene Hunks in: `internal/server/server.go` (`finishTrace`: Urteil Tokens/s nach dem
  Urteil aus #87), `internal/store/traces.go` (Lesen von `trace_tps`),
  `internal/dashboard/{types,convert,performance}.go`, `web/src/lib/types/dashboard.ts`
  (generiert), `web/src/lib/views/{Live,Performance}.svelte`, `docs/ARCHITECTURE.md`
  (Abschnitt Performance), diese Change-Artefakte.

## Was ausdrücklich nicht

- Keine Inhalte in Store, Log oder API, nur Zahlen.
- Kein roter Statuspunkt für Tokens/s: der Punkt bleibt bei #87 (TTFT, Gesamtdauer).
  Tokens/s bekommt eine eigene, dezente Markierung.
- Keine Regression oder Kausalaussage: die Matrix zeigt Zusammenhänge, keine Ursachen.
- Keine Änderung am Schema von `trace_perf` aus #87.
- Kein Umbau des Bereichs „Anfragen“; das ist ein eigenes Issue.

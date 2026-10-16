# Tasks

Gestapelt auf #87/#116 (Performance-Ansicht, Tokens/s-Diagramm).

## 1. Backend

- [x] 1.1 Tests zuerst: `meter` `CacheSavings` (Opus 5.5, Cache-Schreiben zählt nicht, unbekanntes Modell); Store `CostByHour` tabellengetrieben (Stundengrenze, UTC, Tageswechsel, leere Stunden fehlen, ohne Account, ohne Usage, laufend); Handler `hours` bei 1/7/8 Tagen, `cache_savings_nano_usd` und `null` ohne Preis; rot sehen.
- [x] 1.2 `meter.CacheSavings`, `store.CostByHour`, `CostRow.CacheSavingsNanoUSD`, `CostHour`, `CostReport.Hours`; `make generate`.

## 2. Frontend

- [x] 2.1 Tests zuerst (Vitest): Navigation (8 Bereiche, Taste `3`, Umleitung `#/performance` → `#/costs`), Gesamtsumme und Anteile je Account mit Namen aus `/api/accounts`, Kostenkurve je Account mit lückenloser Stundenachse und Tageswerten bei 30 Tagen, Kennzahlen, Umschalter Account/Modell/Client/Tag, Tage ohne Wert ausgeblendet; rot sehen.
- [x] 2.2 Router und `App.svelte`: Bereich „Kosten & Performance“, Alias `performance`.
- [x] 2.3 Helfer in `costs.ts` (Anteile, Kostenkurve, Kennzahlen, Aufschlüsselung), `CostChart.svelte`.
- [x] 2.4 `Costs.svelte`: Dashboard und Aufschlüsselung; `Performance.svelte` als Detailbereich ohne eigenen Abruf, Tokens/s-Diagramm ins Dashboard.
- [x] 2.5 Bestehende Performance- und Kosten-Tests auf die neue Seite umstellen (Inhalte bleiben geprüft).

## 3. Doku und Abschluss

- [x] 3.1 `docs/ARCHITECTURE.md`: Kostenbericht mit Stunden und Cache-Ersparnis, zusammengelegter Bereich.
- [x] 3.2 `make generate`, `cd web && pnpm check && pnpm vitest run`, `go test -race ./internal/...`, `make lint`.
- [x] 3.3 `openspec validate issue-121-kosten-performance-dashboard --strict`.

## 4. Nachbesserung Zeitachse (2026-10-05)

- [x] 4.1 Kostenkurve und Tokens/s-Diagramm immer stündlich auf `hourAxis` (erste Stunde mit Daten bis jetzt, höchstens 7 Tage); `hours` in `/api/costs` für die letzten min(N, 7) Tage, `hours` je Modell in `/api/performance`. Tests zuerst (Go und Vitest).

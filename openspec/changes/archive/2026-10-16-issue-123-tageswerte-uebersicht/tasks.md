# Tasks

Gestapelt auf #121.

## 1. Frontend

- [x] 1.1 Tests zuerst (Vitest, `day-values.test.ts`): `daySpans`, `dayBands` (Überdeckung, jüngster Tag zuerst), `dayCosts` mit Untergrenze, `TpsLine.days`, CostChart mit und ohne Tageswerte, Kosten & Performance mit Tageswerten, Übersicht mit Verlauf und Ausfall des Performance-Berichts; rot sehen.
- [x] 1.2 Helfer `daySpans`, `dayBands`, `labelWidth` (`hours.ts`), `dayCosts` (`costs.ts`), `TpsLine.days` (`perf.ts`).
- [x] 1.3 `CostChart.svelte` und `TpsChart.svelte`: Tageszeile mit Trennlinien.
- [x] 1.4 `TrendCharts.svelte`, eingebunden in `Home.svelte` unter den Abo-Limits; `Costs.svelte` gibt die Tagessummen weiter.
- [x] 1.5 Bestehende Erwartung an `tpsByHour` um `days` ergänzen.

## 2. Abschluss

- [x] 2.1 `make test`, `make lint`, `pnpm check` grün.

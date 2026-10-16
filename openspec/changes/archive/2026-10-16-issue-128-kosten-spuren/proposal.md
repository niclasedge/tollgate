# Proposal

Issue: [#128 Kostenverlauf: je Account eine eigene Linie mit klar unterscheidbarer Farbe](https://github.com/niclasedge/tollgate/issues/128)

## Why

Der Kostenverlauf stapelt die Accounts als Flächen in Blau und Grautönen. Gestapelt zeigt
eine Kurve nicht die Kosten ihres Accounts, und die Grautöne sind kaum zu unterscheiden.

## What Changes

- Die Kostenkurve zeigt je Account eine eigene Linie in einem gemeinsamen Diagramm auf
  gemeinsamer Skala. Die Linien liegen übereinander, nicht gestapelt: jede zeigt die
  Kosten ihres Accounts. (Eine erste Fassung mit getrennten Spuren je Account wurde auf
  Wunsch wieder zusammengeführt.)
- Accounts bekommen in fester Reihenfolge fünf kategoriale Farben (`--c-series-1..5` in
  `app.css`), hell und dunkel mit dem Farbvalidator auf Lesbarkeit bei Farbsehschwäche
  geprüft. Ab dem sechsten Account Grau statt einer wiederholten Farbe. Legenden auf der
  Übersicht und auf Kosten & Performance nutzen dieselben Farben.
- Bewusste Abweichung vom Designgrundsatz „ein Blauton, sonst Grau“: nur für Diagramm-
  serien, auf ausdrücklichen Wunsch.

## Capabilities

### Modified Capabilities

- `frontend-views`: Darstellung der Kostenkurve.

## Impact

- Frontend: `components/CostChart.svelte`, `lib/costs.ts` (`seriesColor` statt `areaStyle`),
  `components/TrendCharts.svelte`, `views/Costs.svelte`, `app.css`, Tests.

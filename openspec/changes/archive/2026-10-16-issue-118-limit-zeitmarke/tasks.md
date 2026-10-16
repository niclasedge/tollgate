# Tasks

## 1. Tests zuerst

- [x] 1.1 `web/src/limit-marks.test.ts`: `elapsedPercent` (5 h, Woche, ohne/ungültiger/abgelaufener Reset, Reset weiter als Fenster); `LimitCard` mit Marke bei 5 h und Woche (Uhr gemockt), ohne Marke bei fehlendem/abgelaufenem Reset, aktive Karte mit Klasse und grüner Outline; `AccountUsage` mit 5-h-Marke und ohne Marke bei abgelaufenem Reset; `AccountCard` aktiv grün umrandet. Rot sehen.

## 2. Umsetzung

- [x] 2.1 `web/src/lib/limits.ts`: `elapsedPercent(reset, windowMs, now)` mit Fensterlängen `FIVE_HOURS_MS` und `WEEK_MS`.
- [x] 2.2 `AccountUsage.svelte`: Wochenmarke über `elapsedPercent`, neue 5-h-Marke mit Text.
- [x] 2.3 `LimitCard.svelte`: Marke für beide Fenster mit tickender Uhr, Outline `--c-ok` für `active`.
- [x] 2.4 `AccountCard.svelte`: Outline `--c-ok` für das aktive Abo.

## 3. Abschluss

- [x] 3.1 `cd web && pnpm check && pnpm vitest run` grün.
- [x] 3.2 `openspec validate issue-118-limit-zeitmarke --strict`.

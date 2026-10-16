# Tasks

## 1. Backend

- [x] 1.1 Test: `days=8` liefert Stunden ab dem ersten der 8 Tage, `days=14` nicht mehr; rot sehen.
- [x] 1.2 `maxHourlyCostDays = 8`, Kommentare und `docs/ARCHITECTURE.md` angepasst.

## 2. Frontend

- [x] 2.1 Tests zuerst (`windows.test.ts`): Fensterkosten ab Startstunde, Untergrenze ohne Preis, ohne laufendes Fenster nichts, Wochenvergleich bis zum selben Wochentag, Sonntag, Änderungstexte, Karten mit und ohne Stundenwerte, Abschnitt „Diese Woche“.
- [x] 2.2 `windowCost`, `weekComparison`, `deltaLabel`, `pointsLabel` in `lib/windows.ts`.
- [x] 2.3 Fensterkosten in `AccountUsage`/`AccountCard`, Abschnitt „Diese Woche“ in `Home.svelte`, `days=14`.

## 3. Abschluss

- [x] 3.1 `make test`, `make lint`, `pnpm check` grün.

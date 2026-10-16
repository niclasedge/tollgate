# Tasks

## 1. Store und Proxy

- [x] 1.1 Migration 0010 mit `seven_day_permille` und `seven_day_reset`; `TraceResult`, `FinishTrace` und `FillRow` erweitert; Store-Test.
- [x] 1.2 Header-Lesen um 7 Tage erweitert, `finishTrace` speichert die Werte; Unit-Test.

## 2. Dashboard-API

- [x] 2.1 `capacity` als reine Funktion mit Tabellentest (Preis je vollem Fenster, Wochenlimit aus Kosten und Wochenverbrauch); `CachedUsage`; im Performance-Bericht; `make generate`.

## 3. Frontend

- [x] 3.1 `capacity.ts` (Prognose, Urteil) mit Vitest; `CapacityCard.svelte` in `Costs.svelte` mit Komponententest.

## 4. Abschluss

- [x] 4.1 `make test`, `make lint`, `pnpm check` grün; deployt.

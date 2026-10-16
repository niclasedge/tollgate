# Proposal

Issue: [#124 Übersicht: Abos nach Verfügbarkeit ordnen, Karten kompakter](https://github.com/niclasedge/tollgate/issues/124)

## Why

Ist das 5-h-Fenster des aktiven Abos voll, soll man auf der Übersicht sofort sehen, welches
Abo übernehmen kann. Bisher stehen die übrigen Abos in Listenreihenfolge, und die Karten
brauchen Zeilen für Name und E-Mail, die Erklärung der Zeitmarke und den Reset.

## What Changes

- Übersicht: aktives Abo zuerst (links), dann Abos mit Wochenkontingent und freiem
  5-h-Fenster (nächster 5-h-Reset zuerst, ohne laufendes Fenster ganz vorn), dann Abos mit
  vollem 5-h-Fenster nach nächstem Reset, dann Abos ohne Wochenkontingent nach Wochen-Reset,
  zuletzt Abos ohne Nutzungsdaten.
- Karten: E-Mail als Titel (Name als Tooltip, ohne E-Mail der Name), die Zeile
  „Strich = jetzt …“ entfällt (die Marke behält Tooltip und Label), der Reset steht rechts
  neben der Prozentzahl. Gilt für `AccountCard`/`AccountUsage` und `LimitCard`.

## Capabilities

### Modified Capabilities

- `frontend-views`: Reihenfolge der Abo-Karten, kompakte Karten.

## Impact

- Frontend: neu `web/src/lib/subscriptions.ts`, `views/Home.svelte`,
  `components/AccountCard.svelte`, `components/AccountUsage.svelte`,
  `components/LimitCard.svelte`, Tests.

## Was ausdrücklich nicht

- Keine Reihenfolge auf der Seite Accounts: dort bleibt die Liste stabil für die Aktionen.

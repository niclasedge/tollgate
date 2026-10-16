# Proposal

Issue: [#125 Übersicht: Kosten je Abo im 5-h- und 7-Tage-Fenster, Wochen-KPIs](https://github.com/niclasedge/tollgate/issues/125)

## Why

Auf der Übersicht sieht man die Auslastung jedes Abos in Prozent, aber nicht, wie viel
Listenpreis im laufenden 5-h- und Wochenfenster schon angefallen ist. Außerdem fehlt ein
Wochenvergleich: wie viel diese Woche bisher verbraucht wurde, verglichen mit der Vorwoche,
gesamt und pro Tag.

## What Changes

- Jede Abo-Karte der Übersicht zeigt unter beiden Fenstern die Kosten und Anfragen des
  Abos im laufenden Fenster. Grundlage sind die Stundenwerte von `/api/costs`, ab der
  Stunde, in der das Fenster beginnt (Reset minus 5 h bzw. 7 Tage).
- `/api/costs` liefert `hours` für die letzten min(N, 8) statt min(N, 7) Tage, damit ein
  laufendes Wochenfenster immer ganz abgedeckt ist.
- Neuer Abschnitt „Diese Woche“: Kosten, Anfragen, Tokens ein/aus, Cache-Trefferquote und
  Gespart der Kalenderwoche (UTC, ab Montag) mit Vorwochenwert bis zum selben Wochentag und
  Änderung; darunter eine Tabelle pro Wochentag mit Vorwoche und der Vorwochensumme.
- Die Übersicht lädt dafür `/api/costs?days=14` statt `days=1`; „Heute“ nimmt nur den
  letzten Tag.

## Capabilities

### Modified Capabilities

- `dashboard-api`: Stundenwerte über 8 Tage.
- `frontend-views`: Fensterkosten auf den Abo-Karten, Wochenvergleich.

## Impact

- Backend: `internal/dashboard/handler.go` (`maxHourlyCostDays`), Kommentar in `types.go`.
- Frontend: neu `web/src/lib/windows.ts`, `views/Home.svelte`,
  `components/AccountCard.svelte`, `components/AccountUsage.svelte`, Tests.

## Was ausdrücklich nicht

- Keine Fensterkosten auf der Seite Accounts.
- Kein automatischer oder manueller Abo-Wechsel; das folgt in #126.

# Proposal

Issue: [#127 Übersicht: Abos mit Woche ab 99 % nach hinten, freie nach niedrigster Woche](https://github.com/niclasedge/tollgate/issues/127)

## Why

Die Reihenfolge aus #124 sortiert freie Abos nach dem 5-h-Reset und behandelt ein Abo mit
99 % Wochenverbrauch als verfügbar. 99 % reicht für keine Sitzung mehr; und von zwei freien
Abos soll das mit mehr Wochenkontingent zuerst kommen.

## What Changes

- Ein Abo gilt ab 99 % Wochenverbrauch im laufenden Fenster als verbraucht: es steht hinter
  allen Abos mit Wochenkontingent, wird nicht als Wechselkandidat angeboten, und als aktives
  Abo bekommt es die rote Umrandung aus #126.
- Abos mit Wochenkontingent und freiem 5-h-Fenster stehen nach niedrigster Wochenauslastung,
  bei Gleichstand nach niedrigster 5-h-Auslastung. Ein abgelaufenes Fenster zählt als 0 %.
- Abos mit vollem 5-h-Fenster bleiben nach nächstem Reset dahinter.

## Capabilities

### Modified Capabilities

- `frontend-views`: Reihenfolge der Abo-Karten und Schwelle für ein verbrauchtes Wochenkontingent.

## Impact

- Frontend: `web/src/lib/subscriptions.ts`, Tests.

Hinweis: Die geänderten Anforderungen entstehen mit `issue-124-abo-reihenfolge` und
`issue-126-manueller-wechsel`; dieser Change wird erst danach archiviert.

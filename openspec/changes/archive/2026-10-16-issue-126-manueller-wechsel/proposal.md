# Proposal

Issue: [#126 Volles Abo: kein automatischer Wechsel, manueller Wechsel auf der Übersicht](https://github.com/niclasedge/tollgate/issues/126)

## Why

Ist das aktive Abo voll, passt der Betreiber oft erst im Hintergrund das Abo an. Ein
automatischer Wechsel kommt dem zuvor. Der Wechsel soll deshalb sichtbar angeboten, aber
erst auf Knopfdruck ausgeführt werden.

## What Changes

- `alerts.auto_switch` ist standardmäßig aus (Binary und `docker-compose.yml`); wer den
  automatischen Wechsel will, setzt `TOLLGATE_ALERTS_AUTO_SWITCH=true`. Ohne ihn meldet
  tollgate die Ablehnung wie bisher mit Kandidat.
- Übersicht: Hat das aktive Abo im laufenden 5-h- oder Wochenfenster 100 % erreicht, wird
  seine Umrandung rot („Limit erreicht“). Das erste freie Abo in der Reihenfolge aus #124
  bekommt eine blau blinkende Umrandung („als Nächstes“) und den Knopf „Account wechseln“,
  der dieselbe Aktivierung wie die Seite Accounts auslöst. Bei reduzierter Bewegung blinkt
  nichts.

## Capabilities

### Modified Capabilities

- `limit-alerts`: Default des automatischen Wechsels.
- `frontend-views`: Signal und Wechselknopf auf der Übersicht.

## Impact

- Backend: `internal/config/config.go`, `internal/config/alerts_test.go`.
- Deployment: `docker-compose.yml`, `docs/DEPLOY.md`.
- Frontend: `lib/subscriptions.ts`, `components/AccountCard.svelte`, `views/Home.svelte`, Tests.

## Was ausdrücklich nicht

- Kein neuer Endpunkt; der Knopf nutzt `POST /api/accounts/{id}/activate`.
- Die Seite Accounts bleibt unverändert.

Hinweis: `limit-alerts` entsteht mit `issue-122-limit-alarm`; dieser Change wird erst nach dessen Archivierung archiviert.

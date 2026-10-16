# Proposal

Issue: [#142 Account-Wechsel sofort erzwingen](https://github.com/niclasedge/tollgate/issues/142)

## Why

Ein vorgemerkter Account-Wechsel wartet, bis alle laufenden Anfragen fertig sind. Lange Streams halten den Wechsel minutenlang auf, gerade wenn das aktive Abo voll ist und sofort gewechselt werden soll.

## What Changes

- `accounts.Gate`: `WaitBegin` gibt einen abbrechbaren Kontext je Anfrage zurück. `Force()` bricht alle aufgenommenen Anfragen mit Cause `ErrForcedSwitch` ab, aber nur bei vorgemerktem Wechsel.
- Proxy: Abbruch vor Antwortbeginn → 503 `overloaded_error` mit `Retry-After`, Trace-Art `account_switch`. Mitten im Stream wird die Verbindung wie bisher abgebrochen (`stream_aborted`).
- API: `POST /api/accounts/switch/force` liefert den `SwitchStatus`, ohne Vormerkung 409.
- Frontend: „Sofort wechseln“ im Wartehinweis der Account-Verwaltung und neben „Account wechseln“ auf der Übersicht, solange Anfragen laufen.

## Capabilities

### Modified Capabilities

- `queued-account-switch`: Wechsel lässt sich während der Wartephase erzwingen.

## Impact

- `internal/accounts/gate.go`, `manager.go`, `internal/server/server.go`, `internal/dashboard/accounts.go`, `web/src/lib/api.ts`, `AccountManager.svelte`, `Home.svelte`, Tests.

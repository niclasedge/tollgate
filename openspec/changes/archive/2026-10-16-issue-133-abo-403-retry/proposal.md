# Proposal

Issue: [#133 Proxy: Abo ohne OAuth-Zugang (403) als wiederholbaren 503 melden](https://github.com/niclasedge/tollgate/issues/133)

## Why

Verliert das aktive Abo den OAuth-Zugang, antwortet der Upstream mit `403 permission_error` (`oauth_not_allowed_for_organization`). Claude Code wertet 403 als endgültig und bricht den Chat ab. Ein Abo-Wechsel in tollgate behebt die Ursache in Sekunden, der Chat ist dann aber schon beendet.

## What Changes

- Ein Upstream-403 auf `/v1/messages` vom Typ `permission_error` mit Fehlercode `oauth_*` wird zu `503 api_error` mit `Retry-After: 15` und `x-should-retry: true`. Claude Code wiederholt 5xx selbst (Standard 10 Versuche, rund 2,5 min) und läuft nach dem Wechsel auf dem neuen Abo weiter.
- WARN-Log `upstream account not allowed` mit Trace-ID, Account und Fehlercode, nie Body. Trace-Fehlerart `account_not_allowed`.
- Jeder andere 403 bleibt unverändert.
- Nicht in diesem Change: die Anfrage in tollgate halten, bis gewechselt ist. Das spart den erneuten Upload, braucht aber eine Freigabe des Gates während der Wartezeit (sonst blockiert der Wechsel). Bei Bedarf eigenes Issue.

## Capabilities

### Modified Capabilities

- `proxy`: Weiterleiten der Pipeline-Ausgabe an CLIProxyAPI.

## Impact

- `internal/server/account_denied.go` (neu), `internal/server/server.go` (`forward`), Tests.

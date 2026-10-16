# Proposal

Issue: [#71 Abo-Limits anzeigen: 5-Stunden-Fenster und Woche aus den Ratelimit-Headern](https://github.com/niclasedge/tollgate/issues/71)

Geht zurück auf GitHub-Issue #71. Keine offenen Abhängigkeiten.

## Why

Im Frontend sieht man Tokens und Kosten, aber nicht, wie viel vom Claude-Abo noch frei ist und wann das 5-Stunden-Fenster zurückgesetzt wird. Ohne diese Anzeige merkt man ein Limit erst, wenn Claude Code abbricht.

## What Changes

- tollgate liest die Header `Anthropic-Ratelimit-Unified-*` passiv aus jeder weitergeleiteten Antwort und hält den letzten Stand im RAM. Es gibt keinen zusätzlichen Aufruf und keinen Zugriff auf den Token.
- `GET /api/status` liefert den Stand als `limits`: Auslastung, Status und Reset für das 5-Stunden-Fenster und die Woche sowie den Zeitpunkt der Messung.
- Die Accounts-Ansicht zeigt beide Fenster mit Balken, Prozent und „zurückgesetzt in …“.

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `meter`: Abo-Limits aus Antwort-Headern.
- `dashboard-api`: `limits` in `/api/status`.
- `frontend-views`: Anzeige der Limits in Accounts.

## Was ausdrücklich nicht

- Kein Speichern in der DB. Nach einem Neustart erscheint der Stand mit der nächsten Anfrage wieder.
- Kein Plan und kein Profil und keine inaktiven Abos: das bleibt in #28.

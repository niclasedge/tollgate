# Proposal

Issue: [#130 Proxy: Body-Lesefehler loggen und Lese-Timeout für langsame Tailnet-Clients anheben](https://github.com/niclasedge/tollgate/issues/130)

## Why

Ein Client im Mobilfunk erreicht tollgate nur über ein Relay. Bei Einbrüchen dauert der Upload eines langen Verlaufs (1–2 MB) länger als 30 s. tollgate bricht dann ab, Claude Code zeigt `400 could not read request body`. Der Abbruch steht in keinem Log.

## What Changes

- Die Frist von 30 s gilt für **Stille** statt für die Gesamtdauer: Jeder gelesene Block verschiebt sie. Ein langsamer, aber stetiger Sender kommt durch; ein Sender, der 30 s nichts schickt, wird weiter abgebrochen.
- Ein fehlgeschlagenes Lesen wird geloggt: Ursache (`idle_timeout` oder `connection`), gelesene Bytes, `Content-Length`, Dauer, Absenderadresse. Nie Inhalt.
- Nicht in diesem Change: Status 408 statt 400. Das braucht eine Änderung in `proxy()`, in der gerade die uncommitteten Änderungen von #114 liegen. Wird nach #114 nachgezogen.

## Capabilities

### Modified Capabilities

- `proxy`: Lesen des Request-Body.

## Impact

- `internal/server/bodyread.go` (neu), `internal/server/server.go` (`serveProxy`, Kommentar zu `bodyReadTimeout`), Tests.

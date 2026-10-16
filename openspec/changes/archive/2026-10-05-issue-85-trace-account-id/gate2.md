# Gate 2
Branch `issue-85/trace-account-id`, gemeinsamer Workspace. Rote Tests vor Umsetzung: `go test ./internal/store ./internal/accounts` meldet fehlendes AccountID/ActiveID. Zusätzlicher Server-Test vor Observer-Verdrahtung rot (AccountResponseObserver fehlt); Frontend vor Anzeige rot (Account-ID und Pro-Account-Tabelle fehlen). Szenarien: Account-Wechsel/Rollback, Stream-Sperre, gleiche ID in Start-/End-Events, gefälschter Client-Header wirkungslos, Identitätsfehler blockiert Forward, getrennte Kosten trotz gleichen Client/Modell/Tag, unbekannte Altdaten und Historie.

## Abschluss

Geprüft im gemeinsamen Workspace für `issue-85/trace-account-id`: Account-/Store-/Dashboard-/Upstream-/Main-Tests mit Race Detector grün, gezielte Server-Tests zur Stream-Zuordnung und zum Identitätsfehler grün. 90 Frontend-Tests und Svelte-Check ohne Fehler/Warnungen grün. Unabhängiges Review ohne konkrete Blocker für #85. Die Historien-Regression entfernt tatsächlich einen vorhandenen Account und prüft erhaltene Trace-Zuordnung und getrennte Verbrauchssummen nach erneutem Öffnen der DB.

`make test` insgesamt nicht grün: parallel geänderte, nicht zu diesem Change gehörende Pipeline-Pinner-Tests kompilierten nicht, Redactor-/Redactor-Server-Tests schlugen fehl. `make lint` meldete Formatierung in fremdem `internal/redact/stage_test.go`. Diese Quellen wurden nicht verändert. `go vet` für die berührten Backend-Pakete grün. Kein eigenständiger Branch-/CI-Nachweis.

`just setup` erfolgreich, beide Container gesund. Zehn zuletzt geprüfte echte Anfragen haben Account-ID; Start-/Abschluss-Zuordnung und Verbrauch sind über die API vorhanden. Neu zugeordnete und historische unbekannte Kosten-Gruppen bleiben getrennt. Kein automatischer Commit, Push, PR oder Merge; Archivierung erst nach Merge.

# Gate 2

## Validierung und Betrieb

- Gezielte Race-Regressionen für Gate, Queue, Abbruch, laufende exklusive Operation und wartende Proxy-Anfragen grün. `make test` bestätigt Accounts, Server, Store und Upstream sowie alle 91 Frontend-Tests; Svelte-Check: 0 Fehler, 0 Warnungen.
- `go vet ./cmd/tollgate ./internal/accounts ./internal/server ./internal/store ./internal/dashboard ./internal/upstream` grün.
- Gesamtlauf bleibt wegen paralleler Redactor-/Rehydrate-Änderungen rot (`/tmp/tollgate-queued-switch-full.log`). `make lint` meldet fremde Formatierung in `internal/redact/rehydrate_stream_test.go` (`/tmp/tollgate-queued-switch-lint.log`). Diese Änderungen wurden nicht verändert.
- Unabhängiger Account-Review: keine Spec-/Code-Blocker; zunächst fehlender Gate-2-Nachweis anschließend ergänzt.
- `just setup` erfolgreich; beide Container healthy. Die neu gebaute Accounts-Oberfläche lädt; Live-Status-API liefert HTTP 200 und zählt zwei laufende Anfragen. Automatischer Drain, Aktivierung und anschließende neue Account-ID wurden kontrolliert in Integrationstests geprüft. Kein zusätzlicher produktiver Wechsel auf den nahezu ausgeschöpften inaktiven Account durchgeführt.
- Prüfung im gemeinsamen Workspace; keine Behauptung einer isolierten Branch-CI. Kein Commit, Push oder PR angelegt.

## Roter Vorlauf

Branch `issue-88/queued-account-switch`, gemeinsamer Workspace. Vor Umsetzung wurde `go test ./internal/accounts` ausgeführt; rot: Gate.Status und WaitBegin fehlen in queued_switch_test.go (Kompilierfehler). Tatsächliches Log lokal unter /tmp/tollgate-queued-switch-red.log; die fehlenden Methoden waren vor Implementierung nicht vorhanden. Danach Gate/Manager/Proxy umgesetzt. Zusätzliche Regressionen prüfen Drain, ID-Auflösung während Wartens, Kontext-Abbruch, Konkurrenz, laufende exklusive Operation, wartende Proxy-Anfrage und Frontend-Fortschritt mit erst später aufgelöster Aktivierungsantwort. Für diese nachträglich ergänzten Szenarien wird kein eigener voriger roter Lauf behauptet.

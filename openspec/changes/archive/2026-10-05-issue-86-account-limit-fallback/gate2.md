# Gate 2
Branch `issue-86/account-limit-fallback`, gemeinsamer Workspace. Vor Korrektur `go test ./internal/accounts` rot (ObserveUsage/ObserveLimits fehlen). Regression: HTTP 429, dann valide Antwort-Limits; korrekte Karte ohne erneuten API-Aufruf, anderer Account bleibt unbekannt, ältere Antwort überschreibt keine neuere, gespeicherter Messstand überlebt Manager-Neustart.

## Abschluss

Geprüft im gemeinsamen Workspace für `issue-86/account-limit-fallback`: Account-Regressionen mit Race Detector grün. Zusätzlich abgesichert: mehr als fünf Sekunden Mutex-Wartezeit ohne weiteren Usage-Read vor Neustart, begrenzte Wiederholung temporärer Schreibfehler, Teilantwort erhält Woche samt Messzeitpunkt. Jeder Limit-Zeitraum trägt bei Bedarf seinen eigenen Messzeitpunkt. Unabhängiges Review fand Timeout vor Mutex und Verlust durch Teilantwort; beide korrigiert, Nachprüfung ohne verbleibende konkrete Blocker. Frontend (90 Tests, Check ohne Fehler/Warnungen) grün.

Vollständiger Workspace-Test und Lint wegen fremder Pipeline-/Redactor-Änderungen nicht grün; Einzelheiten siehe #85 Gate 2. `just setup` erfolgreich, beide Container gesund. Aktiver Account zeigt aus echten Antworten 98 % Wochenverbrauch und Reset-Datum, ohne Usage-Fehlermeldung. Der beim Wechsel gesicherte andere Account behält seine eigene Zuordnung. Keine Tokens, Nutzernamen oder Payloads in diesen Belegen. Kein eigenständiger Branch-/CI-Nachweis, automatischer Commit, Push, PR oder Merge.

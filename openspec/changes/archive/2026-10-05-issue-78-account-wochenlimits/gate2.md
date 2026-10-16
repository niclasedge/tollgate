# Gate 2
Branch `issue-78/account-wochenlimits`, gemeinsamer Workspace. Vor Umsetzung: `go test ./internal/accounts` rot (UsageSnapshot, fetchUsage, Usage fehlen), `pnpm test -- src/account-usage.test.ts` rot (AccountUsage.svelte fehlt). Szenarien prüfen getrennte Account-Werte, rotierte aktive Tokens, unveränderten auth-dir, Cache nach Neustart, sichere gedrosselte Fehler und Wochenposition bei 50 % vergangener Zeit. Frontend prüft auch erschöpftes Budget, fehlende Daten und überschrittenen Reset.

## Abschluss

Geprüft im gemeinsamen Workspace für `issue-78/account-wochenlimits`: `make test` grün (87 Frontend-Tests, Svelte-Check ohne Fehler/Warnungen, `go test -race ./...`), `make lint` grün. Unabhängiges Review fand eine reaktive Abfrageschleife und ein falsches Verfügbarkeits-Signal bei fehlendem Reset; beide korrigiert und durch Regressionstests abgesichert. Nachprüfung ohne verbleibende konkrete Blocker.

`just setup` erfolgreich, beide Container gesund. Live-Read beider gespeicherten Accounts ohne Änderung des aktiven Accounts; auth-dir enthält weiterhin genau eine Token-Datei. Aktiver Account: 2 % Wochenverbrauch, 98 % Rest, 20 % Zeitfortschritt, Reset-Datum vorhanden. Inaktiver Account: tatsächliches HTTP 403 vom Usage-Endpunkt; die Karte zeigt korrekt unbekannte Usage und verweigerten Zugriff. Vollständige Payloads oder Tokens wurden nicht gespeichert. Nach weiterem Container-Neubau bleibt der erfolgreiche Messstand erhalten. Mobil (390 px) ohne horizontalen Überlauf, hell/dunkel und Desktop kontrolliert. Doppelte globale Limits im Accounts-Bereich bei vorhandener Account-Verwaltung ausgeblendet.

Der inaktive Account benötigt außerhalb dieses Changes eine Berechtigungs-Klärung bei Anthropic; kein behaupteter Live-Wert bei abgewiesenem Read. Keine eigenständige Branch-/CI-Prüfung, kein automatischer Commit, Push, PR oder Merge. Archivierung erst nach Merge.

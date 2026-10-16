# Gate 2

Branch `issue-28/account-wechsel`, gemeinsamer Workspace. Vor Implementierung: `go test ./internal/accounts ./internal/cryptobox` rot: Open, Gate, NewOAuth fehlen. Tests prüfen Verschlüsselung mit AAD, fremden State, Ablauf, Replay, sicheren Fehler und laufenden Stream. Manager-Szenarien vor Manager-Implementierung hinzugefügt: rotierte Tokens, Rollback, Leckprüfung und Ein-Datei-Invariante.

Frontend vor Implementierung: `pnpm test -- src/accounts.test.ts` rot (AccountManager.svelte fehlt). Nach Implementierung: 67 Frontend-Tests grün, Svelte-Check ohne Fehler/Warnungen.

## Abschlussprüfung

Geprüft im gemeinsamen Workspace für `issue-28/account-wechsel` mit der CPA-Erweiterung aus `issue-77/cpa-refresh-barriere`: `make test` grün (67 Frontend-Tests, Svelte-Check ohne Fehler/Warnungen, `go test -race ./...`), `make lint` grün. Unabhängiges Review nach Korrekturen ohne konkrete Blocker. `just setup` erfolgreich; beide Container gesund. Zweiter Account über PKCE gespeichert, manuell aktiviert, nach Container-Neubau weiterhin aktiv. Beispielaufruf mit dem ausdrücklich gewünschten `claude-sonnet-5-5`: HTTP 200. Mobile Ansicht ohne horizontalen Überlauf.

Die frühere Haiku-Probe ist kein Nachweis für Sonnet: verkürzte Haiku-ID wurde abgewiesen, datierte Haiku-ID lieferte OAuth-Verbot; Sonnet funktioniert mit dem aktiven Account. Ein Replay des letzten echten Calls ist mangels gespeicherter Request-Payload offen; Traces enthalten nur Metadaten und Hashes, Request-Logging ist ausgeschaltet. Kein eigenständiger Branch-/CI-Nachweis, kein Push, PR oder Merge. Archivierung erst nach Merge.

## Why
In Live setzt die Spalte „Gespart“ ein festes `−` vor den formatierten Wert. Verlängert die Redaction den Text, ist `saved_tokens` negativ und es erscheint „−-12“. Issue: https://github.com/niclasedge/tollgate/issues/91.
## What Changes
Neue gemeinsame Formatierfunktion `formatSavedDelta` in `web/src/lib/format.ts`: positive Ersparnis bleibt „−12“ (Tokens weniger), negative Ersparnis erscheint als „+12“ (Mehrverbrauch), 0 oder fehlend bleibt „–“. Live nutzt sie statt der Inline-Formatierung. Übersicht und Kosten zeigen die Ersparnis ohne Präfix über `formatTokens` und haben daher kein doppeltes Vorzeichen; sie bleiben unverändert.
## Capabilities
### Modified Capabilities
- `frontend-views`: Vorzeichen der Ersparnis in Live.
## Impact
Branch `issue-91/negative-ersparnis`, gemeinsamer Workspace. Dateien: `web/src/lib/format.ts`, `web/src/lib/format.test.ts`, `web/src/lib/views/Live.svelte`, diese Artefakte. Keine API- oder Backend-Änderung.

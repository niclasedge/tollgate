# Fixtures

Alle Fixtures sind bereinigt: das Repo wird public. Keine Secrets, Pfade, Nutzernamen,
Hostnamen, E-Mail-Adressen, System-Prompts oder echte IDs.

## claude-code-2.1.289-messages.json

Eine echte `POST /v1/messages`-Anfrage von Claude Code 2.1.289 (macOS), aufgezeichnet
mit einem lokalen Wegwerf-Mock-Server (Antwort 400), leerem `CLAUDE_CONFIG_DIR` und
leerem Arbeitsverzeichnis, ohne echte Zugangsdaten. Der Prompt war `hi`.

Bereinigung:

- Header: nur `Content-Type`, `User-Agent`, `X-App`, `Anthropic-Version`,
  `Anthropic-Beta` (auf zwei stabile Einträge gekürzt) und `X-Claude-Code-Session-Id`.
  Weggelassen: `Authorization`, `X-Stainless-*`, Host, Verbindungs-Header.
- Body: nur `model`, `messages` (ein Text `hi`), `metadata`, `max_tokens`, `stream`.
  Weggelassen: `system`, `tools`, `thinking`, `context_management`, `safeguards` (enthalten
  Pfade und Nutzerkennung) sowie alle Hinweis-Textblöcke.
- IDs: `session_id` (Header und `metadata.user_id`) und `device_id` sind feste
  Fake-Werte (`00000000-0000-4000-8000-000000000001`, 63 Nullen plus `1`).
- Pfad ohne Query (`?beta=true` an der Originalanfrage).

## redact-dry-run-messages.json

Synthetische Anfrage für den Dry-Run der Redactor-Konfiguration (#15), nicht aufgezeichnet.
Alle Werte sind erfunden: „Projekt Falke“ und „Muster GmbH“ als Wortliste,
Kundennummern im Format `KD-` plus sechs Ziffern, eine Adresse und ein Datenbank-Host unter
der reservierten Domain `.invalid`, ein Platzhalter-Passwort `fakepw-0000`. Sie deckt
User-Text, `tool_use.input` und `tool_result` mit Zeilenumbruch in einem Mehrwort-Eintrag ab.

## multi-turn-session.json

Synthetische Anfrage für die Phase-2-Abnahme (#17), nicht aufgezeichnet. Sie ist der dritte
Turn einer erfundenen Claude-Code-Sitzung, so wie der Client ihn schickt: Assistant-Text und
`tool_use.input` enthalten die zuvor zurückübersetzten Werte im Klartext, das Thinking die
Platzhalter mit einer erfundenen Signatur (ADR-0006). Alle Werte sind erfunden: Adressen unter
`.invalid`, ein Kanarien-Key `sk-ant-api03-ACCEPTCANARY…`, ein Dummy-JWT, das Passwort
`fakepw-0000`, „Projekt Falke“ und die Kundennummer `KD-314159`. Sie deckt `tool_result` als
String und als Block-Array, eine Allowlist-Adresse und die festen Fake-IDs
(`…000000000002`) ab. Der Abnahmetest `cmd/tollgate/acceptance_test.go` führt pro Fixture die
erwarteten Geheimnisse; eine neue Fixture braucht dort einen Eintrag.

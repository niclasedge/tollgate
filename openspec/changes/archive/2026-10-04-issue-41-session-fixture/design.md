# Design

## Verifizierte Quelle (Claude Code 2.1.289)

Aufgezeichnet mit `claude -p "hi"` gegen einen lokalen Mock (127.0.0.1, Antwort 400), leerem Config-Verzeichnis, ohne Zugangsdaten. Der Pfad lautet `/v1/messages?beta=true`; `r.URL.Path` enthält die Query nicht.

Beide Quellen sind gesetzt und tragen dieselbe ID:

- Header `X-Claude-Code-Session-Id: <uuid>`.
- `metadata.user_id` ist ein **JSON-Objekt als String**: `{"device_id":"<64 hex>","account_uuid":"","session_id":"<uuid>"}`. Die Legacy-Form `user_..._session_<uuid>` kam nicht vor; sie bleibt als Rückfall bestehen.

Folgerungen:

- ADR-0006 (Header zuerst, sonst `metadata.user_id`) stimmt; keine neue ADR, kein Codewechsel.
- Die Rangfolge ist praktisch der Header; `metadata.user_id` ist der Rückfall, falls ein Zwischenglied den Header entfernt. Der Test deckt beide Pfade ab (Header entfernt).
- `device_id` ist ein Geräteidentifikator und `account_uuid` kann gefüllt sein: `metadata.user_id` darf nie roh in Logs, DB oder Frontend landen. Intake speichert nur den SHA-256-Schlüssel.
- Der User-Agent der Aufnahme nennt `sdk-ts, agent-sdk`; Header und Metadata-Form entsprechen dem, was die interaktive CLI derselben Version nutzt, wurde aber nur über den `-p`-Pfad aufgezeichnet.

## Fixture

`testdata/fixtures/claude-code-2.1.289-messages.json` (Method, Pfad, Header, Body); Bereinigung siehe README dort.

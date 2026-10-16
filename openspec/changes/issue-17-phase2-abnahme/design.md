# Design

## Ein Binary, ein Mock-Modell

Die Abnahme prüft die Verdrahtung, nicht einzelne Stufen. Deshalb laufen alle drei Tests über
`serve` aus `cmd/tollgate` (wie `TestRedactSwitchWiring`): Konfigurationsdatei mit
`redact.enabled: true` und `log.level: debug`, Store in einem Temp-Verzeichnis, Admin-Token und
Krypto-Schlüssel werden beim Start erzeugt, ein Client entsteht über `POST /api/clients`.
CLIProxyAPI ersetzt ein `httptest`-Server.

Das Mock-Modell antwortet deterministisch aus der Anfrage:

- **Thinking**: `Ich sehe ` plus alle Platzhalter der Anfrage, dazu eine feste Signatur.
  Thinking wird nie zurückübersetzt (ADR-0006), der Client sieht hier also Platzhalter.
- **Text**: `Zitat: ` plus die redigierte letzte Nutzernachricht (Text und `tool_result`).
- **tool_use**: `{"command":"printf '%s\n' <Zitat>"}` als `input_json_delta`.
- Deltas zu 7 Bytes, damit jeder Platzhalter über Event-Grenzen läuft; jedes SSE-Event geht in
  zwei Writes mit Flush, damit auch die Rahmen über Chunk-Grenzen laufen.

Das Zitat ist bewusst die Nutzernachricht im Kontext: So bleibt jeder Platzhalter in dem
Umfeld, in dem der Detektor ihn gefunden hat (z. B. das Passwort innerhalb der
Verbindungs-URL). Den Fall ohne Kontext beschreibt der Abschnitt Befund.

Der Testclient baut daraus, was Claude Code im nächsten Turn schickt: Thinking und Signatur
unverändert, Text und Tool-Input so, wie er sie empfangen hat (Klartext), serialisiert ohne
HTML-Escaping wie `JSON.stringify`.

## Cache-Test

Sitzung A (Header `X-Claude-Code-Session-Id`) läuft vier Turns. Jeder Turn hängt eine
Nutzernachricht an (Tool-Ergebnis des vorigen Turns plus Text) und schickt den ganzen
Verlauf. `messages` steht im Body zuletzt, damit das Ende der vorigen Nachrichten ein
Byte-Offset ist. Geprüft wird:

1. Upstream-Body von Turn k ohne das schließende `]}` plus `,` ist Byte-Präfix von Turn k+1.
2. Die erneut redigierte Assistant-Nachricht upstream ist gleich der Ausgabe des Modells
   (Text, Tool-Input, Thinking, Signatur): Das Modell sieht seine eigene Antwort wieder.
3. Erste Werte behalten ihre Nummer (`email_1`, `api_key_1`), neue Werte bekommen höhere.
4. Der Client sieht Klartext in Text und Tool-Input, Platzhalter nur im Thinking.
5. Nach Turn 2 wird per `POST /api/config/import` (`activate: true`) eine Version mit
   eigener Regel `KD-\d{6}` und Wortliste aktiv. Sitzung A bleibt laut Trace-Detail auf
   Version 1 (eine KD-Nummer in Turn 3 geht unverändert durch, der Präfix bleibt gleich),
   eine neue Sitzung B nutzt Version 2 (`customer_id_1`, `word_1`, `word_2`).
6. Kein erfundenes Geheimnis erreicht den Upstream.

## End-to-End mit Fixtures

Der Test aktiviert die Version mit Regel und Wortliste und schickt jede Datei aus
`testdata/fixtures/` mit ihren Headern. Eine Tabelle im Test nennt pro Fixture die
Geheimnisse und die Allowlist-Werte; eine Fixture ohne Eintrag lässt den Test scheitern, damit
neue Fixtures nicht ungeprüft bleiben. Geprüft wird: Upstream ohne Geheimnisse (auch in
JSON-escapter Form), Allowlist-Werte unverändert, Fixture ohne Geheimnisse bytegleich, Client
bekommt das Zitat im Klartext zurück (Text und Tool-Input), Thinking und Signaturen der
Fixture bytegleich upstream.

Die neue Fixture `multi-turn-session.json` ist der dritte Turn einer erfundenen Sitzung, so
wie Claude Code ihn schickt: Assistant-Text und `tool_use.input` enthalten die
zurückübersetzten Werte im Klartext, das Thinking die Platzhalter mit Signatur.

## Leck-Test

Nach Cache-Sitzung und Fixtures durchsucht der Test nach festen Kanarienwerten (Mails, Key,
Passwort, JWT, Wortlisten-Einträge und ihre Teile):

- die Dateien `t.db*` (Datenbank, WAL, SHM; mindestens zwei Dateien müssen existieren),
- die gesamte Log-Ausgabe auf Debug-Level,
- den Live-Stream `GET /api/traces/events` während des gesamten Verkehrs,
- `GET /api/traces`, jedes `GET /api/traces/{id}`, `/api/redactions`, `/api/costs`,
  `/api/status`, `/api/clients`, `/api/config/versions` und jede Version,
  `/api/config/export` (aktive und Version 2, ohne Wortliste), `POST /api/config/dry-run`
  und `POST /api/config/test`.

KD-Nummern sind keine Kanarienwerte: Eine gepinnte Sitzung redigiert sie mit Version 1 nicht,
und `KD-000000` steht als Beispiel in der Konfiguration. Der ausdrückliche Export mit
`words=plain` ist ausgenommen; er zeigt die Wortliste gewollt nach Warnung.

## Gate 2 und Rot

Die Tests prüfen vorhandenes Verhalten. Rot war vor dem Anlegen der Fixture
`multi-turn-session.json`: Die Fixture-Tabelle des Tests verlangt sie. Die drei
Abnahmeszenarien selbst waren gegen den vorhandenen Code ohne Produktivänderung grün.

## Befund: Platzhalter ohne Kontext

Eine Gegenprobe (Mock-Modell schreibt zusätzlich alle Platzhalter einzeln, z. B.
`<redacted:connection_password_1>` ohne die URL drumherum) zeigt eine Lücke: Der Client
bekommt `fakepw-0000` zurück, schickt es im nächsten Turn als Assistant-Text, und der
Detektor erkennt das Passwort ohne `postgres://…@` nicht. Folge: Klartext geht upstream, und
die erneut redigierte Assistant-Nachricht weicht von der Modellausgabe ab. Betroffen sind alle
Regeln, die nur eine Gruppe im Kontext ersetzen (`conn_string`, `generic_assign`,
`bearer_token`, `auth_header`, `client_secret`, `ps_secret_field`, `azure_cred_guid`,
`cert_thumbprint`). Die Byte-Präfixe bleiben trotzdem stabil, weil die Redaktion
deterministisch ist.

Ein Fix (z. B. bekannte Werte derselben Anfrage überall ersetzen) ändert die
Nummerierungsregel aus ADR-0006 und braucht Entscheidungen zu Mindestlänge und Wortgrenzen. Er
gehört deshalb nicht in diesen Abnahme-Change, sondern in #97.

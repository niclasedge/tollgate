# ADR-0009: Authentifizierung, Schlüssel und Geheimnisse

- **Status:** Angenommen (Freigabe Niclas, 2026-10-04, PR #26)
- **Datum:** 2026-10-04
- **Issue:** #1

## Kontext

Harte Regeln aus `CLAUDE.md`:
- *„In Logs, DB und Frontend landen nur redigierte Daten, nie Klartext-Geheimnisse.“*
- *„tollgate bindet nur an die Tailnet-IP, CLIProxyAPI nur an `localhost`.“*

Weitere Vorgaben:
- Client-Keys werden pro Client erzeugt, gesperrt und rotiert, mit Setup-Snippet.
- Das Frontend ist nur aus dem Tailnet erreichbar, zusätzlich mit Admin-Token.
- Die Wortliste wird verschlüsselt gespeichert, maskiert angezeigt und nie geloggt.

Claude Code schickt den Key aus `ANTHROPIC_AUTH_TOKEN` als `Authorization: Bearer …`. Ein
Key aus `ANTHROPIC_API_KEY` käme als `x-api-key`. Die genauen Variablen dokumentiert #11 in
`docs/CLIENT_SETUP.md`.

## Entscheidung

### Netzwerk

| Prozess | Bindung | Durchsetzung |
|---|---|---|
| tollgate | konkrete Tailnet-IP | Startprüfung (ADR-0008): Wildcard und Nicht-Tailnet-Adressen führen zum Abbruch |
| CLIProxyAPI | `127.0.0.1` | `server.host: 127.0.0.1` in der Vorlage (ADR-0001). tollgate prüft beim Start, dass `upstream.url` auf Loopback zeigt. In docker-compose wird kein Port veröffentlicht |

### Client-Keys

- **Format:** `tg_` plus 32 Byte aus `crypto/rand`, base64url ohne Padding (46 Zeichen). Das
  Präfix macht Keys in Logs und Secret-Scannern erkennbar.
- **Speicherung:** nur `SHA-256(key)` in der Tabelle `clients`, dazu Name, Status
  (`aktiv`/`gesperrt`), `created_at` und `rotated_at`. Bei 256 Bit Zufall reicht ein schneller
  Hash. Argon2 schützt nur schwache Geheimnisse.
- **Anzeige:** Der Klartext erscheint genau einmal, bei Erzeugung oder Rotation, im
  Setup-Snippet. Danach zeigt das Frontend nur Präfix und die letzten vier Zeichen.
- **Prüfung:** Der Key wird aus `Authorization: Bearer` oder `x-api-key` gelesen, gehasht und
  im RAM-Cache nachgeschlagen. Schreibvorgänge invalidieren den Cache. Der Vergleich läuft
  über `crypto/subtle.ConstantTimeCompare` auf dem Hash.
- **Sperren** wirkt sofort für neue Requests. Laufende Streams werden nicht abgebrochen.
- **Rotation** erzeugt einen neuen Key. Der alte bleibt optional für eine Übergangszeit
  gültig (Standard 0, also sofort ungültig).
- **Fehlerantworten** kommen im Anthropic-Fehlerformat
  (`{"type":"error","error":{"type":"authentication_error",…}}`), damit Claude Code sie
  sauber anzeigt.

### Admin-Zugang

- Das Admin-Token liegt in `admin.token_file` (ADR-0008) und wird beim ersten Start erzeugt,
  falls die Datei fehlt.
- Es schützt `/api/*` und das Frontend. Das Frontend fragt es einmal ab und speichert es im
  `sessionStorage`. Danach schickt es das Token als `Authorization: Bearer`.
- Vergleich in konstanter Zeit.
- Proxy-Endpunkte (`/v1/*`) akzeptieren nur Client-Keys, `/api/*` nur das Admin-Token. Die
  Wege sind strikt getrennt.

### Verschlüsselung der Wortliste

- AES-256-GCM, Schlüssel aus `crypto.key_file` (32 Byte, Rechte 0600, beim ersten Start
  erzeugt).
- Pro gespeicherter Version eine zufällige Nonce. Determinismus ist hier nicht nötig, er
  gilt nur für Transformationen, nicht für die Speicherung.
- Als Additional Authenticated Data dient die Versions-ID, damit sich Chiffrate nicht
  zwischen Versionen vertauschen lassen.
- **Schlüsselverlust** heißt: Wortlisten früherer Versionen sind nicht mehr lesbar. Ein
  Backup der Schlüsseldatei ist in `docs/CLIENT_SETUP.md` bzw. einer Betriebsanleitung
  dokumentiert.
- **Anzeige im Frontend:** maskiert (`P••••••••e`, Länge nicht exakt). Klartext nur nach
  explizitem Aufdecken pro Eintrag, ohne Logging.

### Kein Klartext in Logs, DB und Frontend

| Ort | Maßnahme |
|---|---|
| Traces (DB) | Diffs, Vorschauen und Hashes entstehen ausschließlich aus der **redigierten** Fassung. Trace-Hashes von Ein- und Ausgabe werden über redigierte Bytes gebildet |
| Redact-Stufe | Der Trace speichert Treffer als `{offset, länge, typ, regel}`, nie den Wert |
| Rückweg | protokolliert nur Platzhalter und Anzahl der Ersetzungen. Die rückübersetzte Antwort wird nicht gespeichert |
| Logs (`slog`) | Request- und Response-Bodys werden nie geloggt. Ein `slog.Handler`-Wrapper maskiert Attribute mit Schlüsseln wie `key`, `token`, `authorization`, `words` |
| Konfigurationsdiffs | Wortliste maskiert |
| Fehlermeldungen | Validierungsfehler zitieren nie Einträge der Wortliste |
| CLIProxyAPI | `request-log: false` (ADR-0001) |

Ein Test schickt bekannte Geheimnisse durch die gesamte Pipeline. Danach durchsucht er die
SQLite-Datei, die Log-Ausgabe und alle `/api/*`-Antworten nach diesen Werten. Es darf keinen
Treffer geben.

## Alternativen

- **mTLS oder Tailscale-Identität statt Client-Keys:** verworfen für Phase 1. Der Auftrag
  verlangt, dass ein Client nur URL und Key setzt. Die Tailscale-Identität (`whois`) kann
  später als zusätzlicher Faktor dazukommen.
- **Argon2 für Client-Keys:** unnötig bei Zufallskeys mit hoher Entropie, und jeder Request
  würde langsamer.
- **SQLCipher für die ganze DB:** braucht cgo und ist auch nicht nötig, weil die DB laut
  Konzept nur redigierte Daten enthält. Verschlüsselt wird gezielt die Wortliste.

## Konsequenzen

- **#8** setzt Client-Keys und Admin-Token um.
- **#15** setzt die Verschlüsselung der Wortliste um.
- **#4** und **#13** setzen die Regel „nur redigierte Daten“ im Store bzw. in der Pipeline um.
- **#17** enthält den Leck-Test über DB, Logs und API.

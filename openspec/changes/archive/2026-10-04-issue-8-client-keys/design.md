# Design

## Context

ADR-0009 legt Format, Hash, Admin-Token und Fehlerformat fest. Der Store bietet `CreateClient`, `ClientByKeyHash`, `ListClients`, `SetClientStatus`, `RotateClientKey`. `pipeline.Intake` übersetzt `ErrInvalidKey` in die Anthropic-Fehlerantwort 401, andere Fehler in 500.

## Decisions

- **Kein Cache.** ADR-0009 erwähnt einen RAM-Cache mit Invalidierung. Hier liest der Verifier pro Request die Zeile über den Hash-Index (lesender Pool). Das erfüllt „sofort abgewiesen“ ohne Invalidierungslogik; bei wenigen Clients ist die Last vernachlässigbar. Ein Cache kann später mit Invalidierung ergänzt werden, ist aber nicht nötig (YAGNI).
- **Konstante Zeit.** Das Nachschlagen erfolgt über den SHA-256 des Keys, der Vergleich zusätzlich über `subtle.ConstantTimeCompare` auf den Hashes. Das Admin-Token wird über SHA-256-Digests verglichen, damit auch die Länge nicht abfließt.
- **Formatprüfung vor dem Lookup:** `tg_` plus 43 base64url-Zeichen, sonst `ErrInvalidKey` ohne DB-Zugriff.
- **Hinweis:** `tg_…` plus die letzten vier Zeichen (Länge 8 Zeichen, innerhalb des Store-Limits von 16).
- **Client-ID:** `c_` plus 16 Hex-Zeichen aus `crypto/rand`.
- **Rotation:** neuer Key, alter Hash wird überschrieben und ist sofort ungültig (Übergangszeit 0). Antwort enthält den neuen Klartext-Key genau einmal. Ein gesperrter Client bleibt beim Rotieren gesperrt.
- **Sperren:** `POST /api/clients/{id}/block`. Entsperren ist nicht gefordert und fehlt bewusst; der Store unterstützt es, die Route kann später folgen.
- **Routen:** `POST/GET /api/clients`, `POST /api/clients/{id}/block`, `POST /api/clients/{id}/rotate`. Alle hinter `RequireAdmin`; Client-Keys öffnen die Admin-API nicht.
- **Admin-Token-Datei:** `LoadOrCreateAdminToken(path)` erzeugt bei fehlender Datei 32 Byte Zufall (Modus 0600, `O_EXCL`), lehnt Tokens unter 32 Zeichen ab, nennt in Fehlern nie den Inhalt.
- **Logging:** optionaler `*slog.Logger`; es werden nur Client-ID und Operation geloggt, nie Keys, Hashes oder Tokens. Antworten tragen `Cache-Control: no-store`.
- **Typen:** einfache Structs mit JSON-Tags, Zeitstempel als RFC-3339-Strings und Status als String, damit tygo ohne Sondertypen generiert. Die Aufnahme von `internal/auth` in `tygo.yaml` erfolgt durch den Koordinator.
- **Entkopplung:** `ClientStore` ist ein kleines Interface über `*store.Store`.

## Risks / Trade-offs

- Pro Request ein Lesezugriff: akzeptiert, siehe oben.
- `rotate` liest den Client danach über den neuen Hash; zwischen Rotation und Lesen könnte eine zweite Rotation liegen, dann liefert der Lesezugriff `404`. Bei einem einzelnen Admin vernachlässigbar.
- `block` liest über `ListClients`, da der Store kein `ClientByID` bietet (siehe Bericht).

# Design

## Context

ADR-0003: `modernc.org/sqlite` über `database/sql`, WAL, ein Schreiber, eigener Runner. ADR-0006: `sessions` mit Schlüssel-Hash, Client, Version, Zeitpunkten. ADR-0008: `config_versions` unveränderlich, Rollback = neue Version, Wortliste verschlüsselt, AAD = Versions-ID. ADR-0009: Clients nur als SHA-256, Traces nur aus redigierten Daten, Treffer als `{offset, länge, typ, regel}`.

## Decisions

- **Ein Schreiber, getrennter Lese-Pool:** `w` mit `MaxOpenConns(1)`, `r` mit `query_only`. Nur Dateipfade, kein `:memory:` (je Verbindung eigene DB). Pragmas per DSN (`busy_timeout` zuerst, dann `journal_mode(WAL)`, `foreign_keys(1)`, `synchronous(NORMAL)`).
- **IMMEDIATE-Transaktionen:** Der Schreiber-DSN setzt `_txlock=immediate`. Lesen-dann-Schreiben (`MAX(id)+1`, Migration) nimmt die Schreibsperre sofort und wartet per `busy_timeout`, statt mit `SQLITE_BUSY` zu scheitern, auch wenn zwei Stores oder Prozesse dieselbe Datei nutzen. Die Migration läuft komplett in einer solchen Transaktion; ein zweiter Opener wartet und findet alles angewandt. `Seal` hält die einzige Schreibverbindung und darf den Store daher nicht aufrufen (Deadlock); das ist am Typ dokumentiert.
- **Dateirechte:** Die DB-Datei wird vor dem ersten `sql.Open` per `os.OpenFile(..., 0o600)` angelegt; eine bestehende Datei mit Gruppen-/Weltrechten wird auf 0600 gesetzt, nie gelockert. SQLite übernimmt den Modus für `-wal`/`-shm`.
- **Migrationen:** `migrations/NNNN_name.sql` per `embed`, aufsteigend sortiert, alle ausstehenden in einer IMMEDIATE-Transaktion. Tabelle `schema_migrations(version, name, checksum)` mit SHA-256 des Dateiinhalts, ohne Zeitstempel (deterministisch). Bereits angewandte Versionen werden übersprungen (idempotent); abweichende Prüfsumme oder fehlende Datei für eine angewandte Version ist ein Fehler. Lücken und doppelte Versionen in den Dateien sind ein Fehler.
- **Zeit:** `created_at`, `rotated_at` und `last_seen_at` stammen aus der injizierbaren Uhr (`Options.Now`), `started_at` von Traces und Schwellen für Aufräumen liefert der Aufrufer. Speicherung als Unix-Millisekunden (INTEGER).
- **Klartextschutz durch API-Form:** Die Typen haben keine Felder für Bodies, Werte oder Klartext-Keys. Ein Reflexionstest hält die Feldliste fest, sodass ein neues Feld bewusst im Test freigegeben werden muss. Zusätzlich validiert der Store: Hashes sind SHA-256 (32 Byte bzw. 64 Hex-Zeichen), `KeyHint` höchstens 16 Zeichen (ein vollständiger `tg_`-Key hat 46), `Diff` höchstens 64 KiB, Stufen- und Treffer-Felder sind Zahlen oder kurze Bezeichner (`[A-Za-z0-9_.:/-]`, 1 bis 64 Zeichen; Groß-/Kleinbuchstaben erlaubt). `author` nutzt dieselbe Zeichenklasse, also ohne Leerzeichen und ohne `@`. Wertebereiche: Zeiten und Zähler nicht negativ, `StartedAt` nicht leer, HTTP-Status 100 bis 599.
- **Untrusted Eingaben (`model`, `endpoint`):** Sie stammen vom Client. `model` erlaubt zusätzlich `[` und `]` (`claude-opus-4-1[1m]`). Ein ungültiger Wert verwirft den Trace nicht, sondern wird zu `unknown` normalisiert; der Originalwert wird nie gespeichert (Leckschutz bleibt).
- **Konfigurationsversionen:** `content` ist das Dokument mit maskierter Wortliste, `words_enc` ein opakes Chiffrat. Weil die AAD die Versions-ID ist, vergibt der Store die ID in der Schreibtransaktion (`MAX(id)+1`, ohne AUTOINCREMENT) und ruft optional `Seal(id)` auf. UPDATE und DELETE auf `config_versions` sind per Trigger verboten; ein BEFORE-INSERT-Trigger verweigert zudem ein INSERT auf eine bestehende ID, weil `INSERT OR REPLACE` sonst ohne DELETE-Trigger überschreiben würde (0001 wird direkt geändert, es gibt noch keinen Release). Die aktive Version steht in der Ein-Zeilen-Tabelle `config_active`.
- **Sitzungen:** `PinSession` fügt ein oder behält die bestehende Zeile (die zuerst gepinnte Version gewinnt) und aktualisiert `last_seen_at`. `PurgeSessions` löscht nach Inaktivität (ADR-0006: 7 Tage, Schwelle gibt der Aufrufer vor).
- **Zeit-Rückgaben:** `CreatedAt` wird auf Millisekunden (UTC) gekürzt, damit Rückgabe und späteres Lesen identisch sind.
- **Trace-Liste:** Sortierung `started_at DESC, id DESC` mit Keyset-Cursor `(Before, BeforeID)`; Zeilen derselben Millisekunde gehen nicht verloren. Der Client-Filter wird dynamisch ins WHERE gebaut, damit `traces_client` genutzt wird. Listen sind auf 1000 Zeilen begrenzt (Traces und Konfigurationsversionen).
- **Traces:** `RecordTrace` schreibt Trace und Stufen in einer Transaktion; `FinishTrace` ergänzt Status, Dauer und `usage` nach der Antwort. Treffer je Stufe als JSON-Array von `{offset,length,type,rule}`, Feldreihenfolge fest durch die Struct.
- **Client-Rotation:** ersetzt den Hash und setzt `rotated_at`; der alte Key ist sofort ungültig (ADR-0009, Standard 0).

## Risks

- `x/tools` wird durch `go get` auf v0.50.0 angehoben (tygo-Tool-Abhängigkeit); `make generate` ist im Review zu prüfen.
- `Diff` ist ein vom Aufrufer gelieferter String; der Store kann nicht prüfen, ob er redigiert ist. Das erzwingen Pipeline (#13) und der Leck-Test (#17).

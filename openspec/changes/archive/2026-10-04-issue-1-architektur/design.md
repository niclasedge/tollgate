# Design

## Context

Motivation: siehe `proposal.md`, Abschnitt „Why“. Dieser Change ist reine Dokumentation. Die eigentliche Lieferung sind die Dateien `docs/ARCHITECTURE.md` und `docs/adr/*.md`. Das Design hält deshalb fest:

1. die Entscheidungen, die in die ADRs übernommen werden, auf Basis der gelesenen Quellen,
2. wie die Dokumente aufgebaut sind.

Gelesene Quellen (Stand 2026-10-04):

- **CLIProxyAPI** (`router-for-me/CLIProxyAPI`):
  - Commit `8ef43e4df3b216a42493105d31c2873b69191473`, Modul `…/v8`, `go 1.26.0`.
  - Lizenz MIT. Copyright-Zeilen: „Copyright (c) 2025-2005.9 Luis Pater“ und „Copyright (c) 2025.9-present Router-For.ME“.
- **hook-redactor** (Rust, v0.8.1, `ref/hook-redactor`):
  - Lizenz MIT, „Copyright (c) 2026 Niclas Edge“.
  - Der portierbare Kern sind rund 1.100 Zeilen: `src/patterns.rs`, `src/redactor.rs`, `src/config.rs`, `src/ignorefile.rs`.
- **Headroom** (`ref/headroom`):
  - Lizenz Apache-2.0, „Copyright 2025 Headroom Contributors“, mit NOTICE.
  - Die deterministischen Kompressoren liegen in `crates/headroom-core/src/transforms/`.
- **Lokale Toolchain:** Go 1.26.5, Node 22, pnpm 10, Claude Code 2.1.288.

Drei Befunde prägen das Design:

1. **CLIProxyAPI baut den Request-Body um**, wenn es den Client nicht als Claude Code erkennt (Cloaking):
   - Es ersetzt den System-Prompt, baut `anthropic-beta` neu auf und hebt die TTLs von `cache_control` an.
   - Als Claude Code gilt ein Client nur, wenn alles zutrifft: `X-App: cli`, User-Agent `claude-cli/2.1.x` mit Patch ≥ 280, `anthropic-beta` enthält `claude-code-20250219`, und `metadata.user_id` ist gültig.
   - tollgate muss diese Merkmale deshalb unverändert durchreichen.
2. **Headroom komprimiert nur den jeweils letzten Turn.** Headroom gibt selbst zu, dass das den Cache im nächsten Turn bricht, und gleicht es mit Sitzungszustand aus (`prefix_tracker`). Eine zustandslose Funktion, die bei jedem Turn auf den ganzen Verlauf angewendet wird, vermeidet das Problem von vornherein.
3. **Der Rust-Redactor ersetzt sequenziell**, eine Regel nach der anderen über die Ausgabe der vorigen Regel. Dabei gehen Labels verloren: `api_key=sk-ant-…` wird zu `<redacted>`. Es gibt keine Nummerierung und keine Rückübersetzung.

## Goals / Non-Goals

**Goals:**
- Jede Pflichtfrage aus Issue #1 ist durch ein ADR beantwortet. Jedes ADR nennt die verworfenen Alternativen.
- Jede „Harte Regel“ aus `CLAUDE.md` ist einer konkreten Entscheidung zugeordnet: Wo wird sie technisch durchgesetzt?
- Abweichungen vom Auftrag in `CLAUDE.md` stehen gesammelt in einem Abschnitt von `ARCHITECTURE.md` und sind ausdrücklich zur Freigabe markiert.

**Non-Goals:**
- Kein Code, keine Kopie von CLIProxyAPI. Beides kommt erst mit #2 und #3.
- `docs/CLIENT_SETUP.md` gehört zu #11.
- Keine fertigen Regex-Kataloge und keine fertigen Kompressionsregeln. Die ADRs legen nur Umfang und Prinzipien fest.

## Decisions

### D1 · CLIProxyAPI läuft als eigener Prozess (ADR-0001)

**Einbindung:**
- Die Kopie liegt wie im Auftrag vorgesehen unter `third_party/cliproxyapi/`, aber als **eigenständiges Go-Modul**. Sie wird nie importiert.
- Daraus entsteht ein separates Binary, das nur an `127.0.0.1` gebunden ist.
- Ziel ist, die Kopie gar nicht zu patchen. `PATCHES.md` hält den Commit-Hash fest und begründet jede Abweichung.

**Begründung:**
- Das Claude-Verhalten liegt vollständig in `internal/`: Erkennung, Cloaking, Betas, CCH-Signatur und Executor. Das SDK (`sdk/cliproxy`) bringt also keine zusätzliche Kontrolle.
- Als Bibliothek würde es aber Folgendes mitziehen: pion/webrtc, go-redis, utls und eine Go-Untergrenze von 1.26.
- Für das SDK gibt es keine Stabilitätszusage. Die Doku ist veraltet (sie nennt v6, aktuell ist v8).
- Mit eigenem Prozess heißt Upstream-Update: Kopie austauschen, `config.example.yaml` vergleichen, Smoke-Test.
- Weil die Kopie ein eigenes `go.mod` hat, ignoriert `go test ./...` in tollgate sie automatisch.

**Pflichtkonfiguration für CLIProxyAPI** (Vorlage unter `deploy/cliproxyapi/config.yaml`, ohne Geheimnisse):
- `server.host: 127.0.0.1`
- `access.api-keys` mit genau einem starken Schlüssel für tollgate. Eine leere Liste würde den Zugang komplett öffnen.
- `requests.passthrough-headers: true`, damit die `anthropic-ratelimit-*`-Header ankommen.
- `upstream.claude.disable-claude-cloak-mode: true` als Sicherheitsnetz.
- `observability.logs.request-log: false`
- Management-API mit `secret-key`, `allow-remote: false`, `disable-control-panel: true`

**Regeln für tollgate beim Weiterleiten** (sonst greift Cloaking):
- `User-Agent`, `X-App`, `anthropic-beta`, `anthropic-version`, `X-Claude-Code-*` und `X-Stainless-*` gehen unverändert durch.
- `metadata.user_id` und der Billing-Block in `system[0]` werden nie verändert.

**Aktiver Account:**
- Im auth-dir liegt laut harter Regel genau eine Token-Datei.
- tollgate liest den Status über `GET /v8/management/credentials` und zeigt ihn an. Der Aufruf geht nur an localhost und ist mit Secret geschützt.

**Alternativen:**
- **SDK-Einbettung** (ein Binary, Callback pro Request): verworfen. Kopplung an das Release-Tempo von upstream, schwere Abhängigkeiten, keine Kontrolle über Claude-Interna.
- **Nur ein gepinntes Docker-Image ohne Kopie:** am einfachsten zu aktualisieren, widerspricht aber dem Auftrag „Kopie unter third_party“. Bleibt als Rückfalloption im ADR notiert.

### D2 · Zielstruktur des Repos (ADR-0002)

Der Vorschlag aus `CLAUDE.md` wird bestätigt. Ergänzungen:

- `internal/jsonsplice/`: Bytegenaues Ersetzen einzelner JSON-Strings (siehe D4). Redact und Compress nutzen es gemeinsam.
- `internal/sse/`: SSE-Parser und -Writer. Upstream-Client und Rückweg nutzen sie gemeinsam.
- `internal/auth/`: Client-Keys und Admin-Token. So bleibt `server/` frei von Schlüssellogik.
- `deploy/cliproxyapi/`: Konfigurationsvorlage ohne Geheimnisse.
- `third_party/cliproxyapi/` mit eigenem `go.mod` (siehe D1).

Ansonsten gilt die Struktur aus `CLAUDE.md` unverändert.

### D3 · Bibliotheken (ADR-0003)

| Zweck | Wahl | Verworfen, weil |
|---|---|---|
| Router | stdlib `net/http.ServeMux` (Muster seit Go 1.22), `httputil.ReverseProxy` für die Passthrough-Endpunkte | chi/gin: keine Abhängigkeit nötig |
| SQLite | `modernc.org/sqlite` über `database/sql`, WAL, eingebettete SQL-Migrationen mit eigenem Runner (~50 Zeilen) | mattn/go-sqlite3: braucht cgo. goose: unnötig |
| YAML | `go.yaml.in/yaml/v3` (gepflegter Nachfolger) | `gopkg.in/yaml.v3`: archiviert |
| JSON-Schema | `github.com/santhosh-tekuri/jsonschema/v6` | xeipuuv/gojsonschema: unterstützt keine aktuellen Drafts |
| JSON-Tokenizer | `encoding/json/jsontext` (stdlib, falls in Go 1.26 ohne GOEXPERIMENT verfügbar, sonst das API-gleiche `github.com/go-json-experiment/json`) | gjson/sjson: weniger strikt. `encoding/json` mit Map: verliert Reihenfolge und Bytes |
| Verschlüsselung | stdlib `crypto/aes` + `crypto/cipher` (AES-256-GCM), Schlüsseldatei außerhalb der DB | age: für ein einzelnes Feld überdimensioniert |
| Tests | stdlib `testing` + `testing.F`, `github.com/google/go-cmp`, `golang.org/x/tools/txtar` für den Redactor-Korpus, Golden-Files mit `-update`-Flag | testify: nicht nötig |
| Logging/Tracing | `log/slog`. OpenTelemetry erst in #24 | – |
| Frontend | Svelte 5, Vite, TypeScript, pnpm, `tygo` (Go→TS), CodeMirror 6 mit `@codemirror/lang-yaml` und `codemirror-json-schema`, Vitest. Eigene CSS-Tokens statt UI-Kit | UI-Kits: widersprechen dem Things-3-inspirierten, ruhigen Design |

### D4 · JSON per Byte-Splicing, Determinismus (ADR-0004)

**Grundsatz:**
- tollgate parst `/v1/messages` nie in eine Map und serialisiert nie neu.
- Ein Tokenizer liefert Byte-Offsets der Zielstrings: `text` in User-Inhalten, `tool_result.content` usw.
- Die Ausgabe ist `original[:start] + neu + original[end:]`. Alle anderen Bytes bleiben identisch, auch Feldreihenfolge, Whitespace und unbekannte Felder.

Damit sind zwei harte Regeln strukturell erfüllt: „unbekannte Felder durchreichen“ und „Byte für Byte gleich“.

**Neu kodierte Strings** gehen durch eine eigene, feste Escape-Funktion:
- Escaped werden nur `"`, `\` und Steuerzeichen.
- UTF-8 bleibt roh.

**Determinismus-Regeln** (gelten für alle Stufen, im ADR als Checkliste):
- keine Zeit
- kein Zufall
- keine Map-Iteration ohne Sortierung
- stabile Sortierung
- Ausgabe hängt nur von Eingabe und gepinnter Konfigurationsversion ab

**Fehlerfall:** Ist das JSON ungültig oder schlägt eine Stufe fehl, bricht tollgate mit einer Anthropic-Fehlerantwort ab und leitet nichts weiter, damit kein Klartext upstream geht.

**Alternative:** `json.RawMessage` plus Structs. Verworfen, weil Top-Level-Objekte trotzdem neu serialisiert werden.

### D5 · Redactor-Port: 1:1 vs. neu (ADR-0005)

**1:1 übernommen:**
- **Regex-Texte der eingebauten Regeln.** Alle sind RE2-kompatibel, es gibt keine Lookarounds.
- **False-Positive-Filter:**
  - private IPv4/IPv6
  - Kontext von Versionsnummern (±16 Zeichen)
  - Beispiel-Domains und `noreply@`
  - Test-IBAN
- **Keyword-Escaping**
- **Testfälle:** rund 200 redaktionsrelevante `#[test]`, als txtar-Korpus unter `internal/redact/testdata/` im Format `input` / `must_contain` / `must_not_contain`. Die Grenzfälle aus `tests/red_team.rs` §7 kommen als Tests dazu, die das aktuelle Verhalten festschreiben.

**Zuordnung zu den Schaltern:**

| Schalter | Rust-Regeln |
|---|---|
| `email` | `email` |
| `api_keys` | anthropic, openai, aws, github, slack, stripe, bearer, auth_header, generic_assign, ps_secret, client_secret, azure_cred_guid, cert_thumbprint |
| `jwt` | `jwt` |
| `connection_urls` | `conn_string` |
| `ip_addresses` | ipv4 + ipv6 |
| `private_keys` | `pem_private` |

Nicht portiert:
- Home-Dir/Username, Krypto-Pfade, `internal_hostname`, DE-PII (IBAN, Geburtsdatum, SVN) und die CSV/JSON/env-Modi. Optional später als zusätzliche Schalter, standardmäßig aus.
- Hook-, Path-Guard-, Such- und Klassifikator-Code, ca. 17.000 Zeilen.

**Neu entworfen:**
1. **Erst alle Treffer sammeln, dann auflösen:**
   - Alle aktiven Detektoren laufen auf dem Originaltext.
   - Überlappungen werden aufgelöst nach: Priorität (spezifisch vor generisch), dann längste Spanne, dann frühester Start.
   - Danach wird einmal ersetzt.
   - Damit sind behoben: Labelverlust, erneutes Matchen der eigenen Platzhalter und der Template-Bug (`$1Users`).
2. **Pro Regel eine „Secret-Gruppe“ statt Replacement-Templates.** Präfixe wie `password=` oder `TenantId=` bleiben außerhalb des Platzhalters stehen.
3. **`connection_urls` matcht nur URLs mit Passwort** (`scheme://user:pass@`). Ersetzt wird nur das Passwort. Rust ersetzt dagegen die ganze URL, auch ohne Passwort.
4. **Konfigurierbarkeit:**
   - eingebaute Regeln einzeln schaltbar
   - eigene Regex mit Beispielen, die beim Laden geprüft werden
   - Wortliste mit `case_sensitive` und `whole_word`. Wortgrenzen werden Unicode-bewusst manuell geprüft, weil `\b` in Go nur ASCII kennt.
   - Allowlist mit exakten Werten, die nach der Erkennung greift
5. **Paritätstest:**
   - vergleicht Go und Rust-Binary (`redactor -t pattern --no-log -` in einem leeren temporären Verzeichnis mit `REDACTOR_CONFIG_DIR`, `REDACTOR_IGNOREFILE_PATH` auf nicht existierende Pfade)
   - Labels werden normalisiert (`_n` entfernt)
   - Bekannte Abweichungen aus 1–3 stehen als Allowlist im Test und sind im ADR begründet
   - Der Unterschied zwischen Unicode und ASCII bei `\b`, `\d` und `\s` wird mit Nicht-ASCII-Fällen abgedeckt

**Alternative:** Rust-Binary per FFI oder als Subprozess nutzen. Verworfen: cgo, kein Streaming, keine Konfigurierbarkeit.

### D6 · Platzhalter, Sitzungen und Rückweg (ADR-0006)

**Platzhalter-Grammatik:** `<redacted:TYP_N>`
- `TYP` passt auf `[a-z][a-z0-9_]{0,31}`, `N` ist eine Dezimalzahl.
- Damit ist die maximale Länge bekannt, das begrenzt den Puffer im Stream.

**Nummerierung pro Anfrage, aus der Anfrage selbst abgeleitet:**
- Werte werden in der Reihenfolge ihres ersten Auftretens im gesamten Request-Body nummeriert, pro Typ.
- Ein Verlauf wächst nur nach hinten. Darum ergibt dieselbe Historie in jedem Turn dieselben Platzhalter, und das ganz ohne gespeicherten Zustand.
- Die Zuordnung existiert nur im RAM, und nur für die Dauer der Anfrage einschließlich Antwort-Stream.
- Steht in der Eingabe schon platzhalterförmiger Text, wird seine Nummer übersprungen. So kann ein eingeschleuster Platzhalter keinen fremden Wert freilegen.

**Sitzung** = Client-Key und Sitzungs-ID von Claude Code:
- Quelle ist `X-Claude-Code-Session-Id`. Fehlt der Header, wird `metadata.user_id` geparst. Welche Quelle in der Praxis greift, prüft #5 an einer aufgezeichneten Anfrage.
- Die Sitzung pinnt die Konfigurationsversion. Die Zuordnung Sitzung → Version liegt in der DB und enthält keine Geheimnisse.
- Dadurch übersteht das Pinning auch einen Neustart. Neue Regeln gelten erst für neue Sitzungen.

**Welche Blöcke redigiert werden:**

| Block | Richtung | Behandlung |
|---|---|---|
| User-`text`, `tool_result.content` (String oder Text-Array) | Hinweg | redigiert |
| Assistant-`text`, `tool_use.input` (String-Werte) im Verlauf | Hinweg | **ebenfalls redigiert**. Grund: Die rückübersetzten Klartexte des letzten Turns kehren hier zurück. Ohne erneutes Redigieren würden sie upstream gehen und der Präfix würde vom Modell-Output abweichen. |
| `thinking`, `redacted_thinking`, `signature`, `system` | Hinweg | nie verändert |
| `text_delta`, `input_json_delta` | Rückweg | rückübersetzt. Der Puffer hält pro Content-Block-Index zurück, solange ein angefangenes `<redacted:` noch offen ist. In `input_json_delta` wird der Wert JSON-escaped eingesetzt. |
| `thinking_delta`, `signature_delta` | Rückweg | **nicht** rückübersetzt. Die Signatur gilt für die Fassung mit Platzhaltern. Würde sie geändert, lehnt die API den nächsten Turn ab. |

Unbekannte Platzhalter im Modell-Output bleiben unverändert stehen.

**`count_tokens`:**
- Läuft durch dieselben Stufen Redact und Compress, nur ohne Rückweg.
- Grund: Sonst gingen Geheimnisse im Klartext upstream, und die Zählung würde nicht zu `/v1/messages` passen.

**Upstream-Header:**
- tollgate setzt `Accept-Encoding: identity` für `/v1/messages`, damit der Rückweg nur unkomprimiertes SSE verarbeiten muss.
- Hop-by-hop-Header, `Authorization` und `x-api-key` werden ersetzt (CLIProxyAPI-Key). `Content-Length` wird neu berechnet.

**Alternativen:**
- **Pro Sitzung gespeicherte Zuordnung mit TTL**, wie im Auftrag beschrieben: verworfen.
  - Subagenten teilen sich eine Sitzungs-ID, haben aber unterschiedliche Verläufe. Ein sitzungsweiter Zähler würde dort kollidieren.
  - Nach einem Neustart wäre die Zuordnung weg.
  - Die Ableitung pro Anfrage ist strenger („nur RAM“) und zustandslos.
- **HMAC-basierte Platzhalter:** verworfen. Sie sind nicht nummeriert und schlechter lesbar.

### D7 · Kompression und Headroom-Port (ADR-0007)

**Grundsätze:**
- **Zustandslos über den ganzen Verlauf:**
  - Jeder Turn wendet dieselbe reine Funktion auf jedes `tool_result` an, dessen zugehöriger `tool_use` ein Bash-Aufruf ist.
  - Die Funktion hängt nur von Befehl, Ausgabe und gepinnter Konfigurationsversion ab.
  - Gleiche Eingabe ergibt gleiche Ausgabe, der Präfix bleibt stabil.
  - Kein Prefix-Tracker.
- **Regelauswahl** über `tool_use.input.command`. Wie zusammengesetzte Befehle (`cd x && go test ./...`, Pipes, Env-Präfixe) aufgelöst werden, legt #19 fest.
- **Ersetzt wird nur**, wenn die Ausgabe mindestens N Bytes groß ist und das Ergebnis strikt kleiner wird.
- `is_error` und `tool_use_id` bleiben erhalten.
- Andere Tools (Read, Edit, Grep, Glob, Write, MCP) werden nie angefasst.

**Aus Headroom portiert** (Rust-Quelle `crates/headroom-core/src/transforms/`):
1. **LogCompressor** (`log_compressor.rs`):
   - Level-Klassifikator
   - Stacktrace-Zustandsautomat für Python, JS, Java, Rust und Go
   - Erkennung von Summary-Zeilen
   - Auswahl: erster und letzter Fehler plus Top-N mit Kontext
   - Footer `[N lines omitted: …]`
   - Wird genutzt für die Regeln zu `cargo test`, `go test`, `pytest`, `npm`/`jest` und `make`
   - **Feste Obergrenzen pro Regel** statt `adaptive_sizer`. Dessen zlib-Längenprüfung ist nicht byte-portabel.
2. **Dedupe-Normalisierer:** Hex wird vor Dezimal ersetzt (behebt einen Bug in Headroom).
3. **DiffCompressor:** Kontext kürzen, Obergrenzen für Hunks und Dateien, Summary. Für `git diff` und `git show`.
4. **DiffNoise:** Lockfile- und reine Whitespace-Hunks entfernen. Ohne CCR: tollgate schreibt nur, was entfernt wurde.
5. **LogTemplate:** Läufe gleichartiger Zeilen zusammenfassen, für `npm install`, `docker build` und Fortschrittszeilen.

**Optional:** der grep/rg-Parser aus SearchCompressor.

**Nicht portiert:**
- **Kompress, Magika, CodeCompressor, SmartCrusher, Embeddings:** ML-basiert bzw. nicht deterministisch.
- **CCR mit `headroom_retrieve`-Tool:** hält Zustand und verändert `tools`.
- **ReadLifecycle:** komprimiert Read-Ausgaben, verstößt also gegen eine harte Regel.
- **Prefix-Tracker, Learn/TOIN, Output-Shaper, automatisches `cache_control`:** halten Zustand bzw. verändern System, Tools oder Effort.
- **Token-Gate über tiktoken:** Ein Byte-Gate reicht.

**Fixtures:** `tests/parity/fixtures/diff_compressor/*.json` sind als Golden-Basis brauchbar. Die Log-Fixtures sind fast alle unter 50 Zeilen und bleiben unverändert. Deshalb kommen eigene Golden-Files unter `internal/compress/testdata/` dazu.

**Lizenz:** Apache-2.0 §4. NOTICE übernehmen, Herkunft und Änderungen in `third_party/NOTICE.md` und in den Dateiköpfen vermerken.

### D8 · Konfigurationsschema (ADR-0008)

Es gibt zwei getrennte Ebenen.

**1. Bootstrap-Konfiguration**
- Datei `tollgate.yaml`, jeder Wert per Env-Variable `TOLLGATE_*` überschreibbar. Nicht versioniert, nicht im Frontend.
- Inhalt:
  - `listen` (konkrete IP:Port)
  - `upstream.url`, `upstream.api_key_file`
  - `management.url`, `management.secret_file`
  - `db.path`
  - `crypto.key_file`
  - `admin.token_file`
- **Start wird verweigert**, wenn `listen` eine Wildcard-Adresse ist oder außerhalb der Tailscale-Bereiche (`100.64.0.0/10`, `fd7a:115c:a1e0::/48`) liegt. Ausnahme: das Flag `--dev-allow-any-listen` für lokale Tests.

**2. Laufzeitkonfiguration**
- Versioniert in der DB, per Frontend und YAML pflegbar.
- Ein Dokument, eine Versionsnummer. Gepinnt werden Redact und Compress gemeinsam.
- Das Schema aus `CLAUDE.md` bleibt unverändert gültig und wird nur um `compress:` erweitert:

```yaml
version: 1
builtin: { email: true, api_keys: true, jwt: true, connection_urls: true, ip_addresses: false, private_keys: true }
custom:
  - { name: kundennummer, pattern: 'KD-\d{6}', placeholder: customer_id, examples: ["KD-123456"] }
words: { case_sensitive: false, whole_word: true, list: ["Projekt Falke"] }
allowlist: ["noreply@github.com"]
compress:
  min_bytes: 2048
  rules:
    - { name: go_test, match: ["go test"], strategy: test_log, max_errors: 10, context_lines: 3 }
    - { name: git_diff, match: ["git diff", "git show"], strategy: diff, max_files: 20, max_hunks: 10 }
```

- **JSON-Schema:** `config/schema/tollgate-config.v1.schema.json`. Es ist ins Binary eingebettet und wird dem Frontend für CodeMirror ausgeliefert.
- **Import-Ablauf:**
  1. YAML parsen
  2. gegen das Schema validieren. Fehlermeldung mit Pfad, Zeile und Spalte.
  3. Beispiele der eigenen Regex prüfen
  4. Dry-Run auf `testdata/fixtures/`
  5. Bestätigung
  6. neue Version anlegen
  7. Hot-Swap per `atomic.Pointer`
- **Export:** deterministisch aus Structs mit fester Feldreihenfolge. Die Wortliste wird beim Export im Klartext ausgegeben, mit Warnkommentar im Dateikopf und einem Hinweis vor dem Committen.
- **Versionen:** Jede Änderung erzeugt eine neue Zeile mit Zeitstempel und Diff. Im Diff ist die Wortliste maskiert. Rollback legt eine neue Version mit dem alten Inhalt an. Die Historie wird nie umgeschrieben.

### D9 · Authentifizierung, Schlüssel, Geheimnisse (ADR-0009)

- **Client-Keys:**
  - Format `tg_` + 32 Byte Zufall, base64url.
  - Gespeichert wird nur SHA-256. Bei hoher Entropie reicht das, Argon2 ist nicht nötig.
  - Im Klartext sichtbar nur einmal bei Erzeugung oder Rotation, im Setup-Snippet.
  - Sperren wirkt sofort über einen RAM-Cache, der beim Schreiben invalidiert wird.
- **Admin-Token:** aus einer Datei, Vergleich in konstanter Zeit, gilt nur für `/api/*` und das Frontend.
- **Wortliste:**
  - AES-256-GCM mit zufälliger Nonce pro Version. Bei der Speicherung zählt Determinismus nicht, nur bei der Prompt-Transformation.
  - Der Schlüssel liegt in `crypto.key_file`, mit Rechten 0600. Fehlt die Datei, wird sie beim ersten Start erzeugt.
  - Die Wortliste wird nie geloggt und im Frontend maskiert.
- **Klartext gelangt nirgends hin:**
  - Traces, Diffs und Logs entstehen ausschließlich aus der redigierten Fassung.
  - Hashes von Ein- und Ausgabe werden über die redigierten Bytes gebildet.
  - Der Rückweg protokolliert nur Platzhalter.

### Aufbau der Dokumente

- **`docs/ARCHITECTURE.md`:**
  - Überblick und Datenfluss (Mermaid)
  - Komponenten und Pakete
  - Pipeline-Stufen 0–5 mit Ein- und Ausgabe
  - Sitzungsmodell
  - Netzwerk und Prozesse
  - Matrix der harten Regeln (Regel → ADR → Durchsetzung → Test)
  - **Abweichungen vom Auftrag (zur Freigabe)**
  - Auswirkungen auf die Folge-Issues
- **`docs/adr/NNNN-slug.md`:** Status (`Vorgeschlagen`, nach der Freigabe `Angenommen`), Kontext, Entscheidung, Alternativen, Konsequenzen, Quellenbelege mit Pfaden in den Referenz-Repos und Commit-Hash.
- **`docs/adr/README.md`:** Index.

## Risks / Trade-offs

- **[Claude Code springt auf 2.2.x]** CLIProxyAPI erkennt den Client dann nicht mehr als Claude Code und schaltet Cloaking ein. Das bricht den Cache.
  → `disable-claude-cloak-mode: true`, `header-defaults.user-agent` pflegen, Health-Check in tollgate. Der Health-Check vergleicht den User-Agent der Clients mit der Baseline von CLIProxyAPI und warnt im Frontend.
- **[Fingerprint im Billing-Header]** Claude Code berechnet `cc_version` aus den Zeichen 4, 7 und 20 der ersten User-Nachricht. Redact kann diese Zeichen ändern.
  → Spike in #13 mit echtem Account. Option: Den Billing-Block entfernen, damit CLIProxyAPI ihn über den redigierten Inhalt neu erzeugt (deterministisch).
- **[Assistant-Blöcke werden mitredigiert]** Das geht über die Formulierung „nur User-Text und tool_result“ hinaus.
  → Ausdrücklich zur Freigabe markiert. Der Cache-Test in #17 belegt, dass der Präfix stabil bleibt.
- **[Kopie von CLIProxyAPI im Repo]** Rund 33 MB und 1.600 Go-Dateien in einem Repo, das später öffentlich wird.
  → MIT erlaubt das. Rückfalloption: ein gepinntes Image ohne Kopie (D1).
- **[Unicode-Semantik bei RE2]** `\b`, `\d` und `\s` verhalten sich in Rust und Go unterschiedlich.
  → Korpus mit Nicht-ASCII-Fällen. Abweichungen sind als bewusste Änderung dokumentiert.
- **[Neue Nummerierung bei Compaction]** Wenn Claude Code den Verlauf kürzt, verschieben sich die Nummern.
  → Unkritisch: Der Präfix ändert sich dann ohnehin, und die Zuordnung bleibt innerhalb der Anfrage konsistent.

## Migration Plan

Entfällt, weil es noch keinen Code gibt. Nach der Freigabe:

1. Status der ADRs auf `Angenommen` setzen.
2. Folge-Issues mit `gh issue edit` anpassen.
3. #1 schließen.

## Open Questions

- Docker-Topologie für die Bindung an die Tailnet-IP: Host-Netzwerk oder Tailscale-Sidecar. Alternativ zwei Binaries unter launchd/systemd. Wird in #3 und #11 entschieden, ändert die Architektur nicht.
- Heuristik für die Token-Schätzung vorab (Bytes/4 oder Zählen pro Block). Wird in #7 entschieden.

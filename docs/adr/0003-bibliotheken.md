# ADR-0003: Bibliotheken

- **Status:** Angenommen (Freigabe Niclas, 2026-10-04, PR #26)
- **Datum:** 2026-10-04
- **Issue:** #1

## Kontext

Vorgaben aus dem Auftrag:
- statisches Binary, SQLite ohne cgo
- Frontend per `embed`
- Logging mit `log/slog`
- OpenTelemetry optional

Leitlinie: so wenige Abhängigkeiten wie möglich. Jede Abhängigkeit muss etwas leisten, das
die Standardbibliothek nicht kann.

Lokale Toolchain bei der Entscheidung: Go 1.26.5, Node 22, pnpm 10.

## Entscheidung

### Backend (Go)

| Zweck | Wahl | Stand | Verworfen, weil |
|---|---|---|---|
| HTTP-Router | stdlib `net/http.ServeMux` mit Methoden- und Pfadmustern (seit Go 1.22) | – | chi, gin, echo: keine Funktion fehlt, die eine Abhängigkeit rechtfertigt |
| Passthrough | stdlib `net/http/httputil.ReverseProxy` mit `FlushInterval: -1` | – | Eigenbau ist für die durchgereichten Endpunkte unnötig |
| SQLite | `modernc.org/sqlite` über `database/sql`, WAL-Modus, ein Schreiber | v1.60.x | `mattn/go-sqlite3` braucht cgo |
| Migrationen | eigene eingebettete SQL-Dateien (`embed`), Runner mit ~50 Zeilen, Versionstabelle | – | goose, golang-migrate: zu groß für eine einzelne lokale DB |
| YAML | `go.yaml.in/yaml/v3` | v3.0.x | `gopkg.in/yaml.v3` ist archiviert. `go.yaml.in` ist der gepflegte Nachfolger mit gleicher API |
| JSON-Schema | `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.x | `xeipuuv/gojsonschema` unterstützt keine aktuellen Drafts mehr |
| JSON-Tokenizer mit Offsets | `github.com/go-json-experiment/json` (Paket `jsontext`), auf Commit gepinnt | Pseudo-Version | siehe unten |
| Verschlüsselung | stdlib `crypto/aes` + `crypto/cipher` (AES-256-GCM) | – | `filippo.io/age` ist für ein einzelnes verschlüsseltes Feld überdimensioniert |
| Hashing von Keys | stdlib `crypto/sha256`, `crypto/subtle` | – | Argon2/bcrypt nicht nötig bei 256-Bit-Zufallskeys (ADR-0009) |
| Logging | stdlib `log/slog`, JSON-Handler | – | zap, zerolog: kein Bedarf |
| Tracing | OpenTelemetry SDK, erst in #24, hinter Konfigurationsschalter | – | – |
| Tests | stdlib `testing` inkl. `testing.F`, `net/http/httptest` | – | – |
| Test-Vergleiche | `github.com/google/go-cmp` | v0.7.0 | testify: Assertion-DSL nicht nötig |
| Korpus-Format | `golang.org/x/tools/txtar` | v0.51.x | eigenes Format: txtar ist etabliert und gut diffbar |
| TS-Typen | `github.com/gzuidhof/tygo` (`go tool`-Direktive in `go.mod`) | v0.2.x | Typen von Hand pflegen ist laut Auftrag verboten |

**JSON-Tokenizer, Begründung:**
- Prüfung am 2026-10-04: `encoding/json/jsontext` ist in Go 1.26.5 **nur** mit
  `GOEXPERIMENT=jsonv2` verfügbar. `go doc encoding/json/jsontext` schlägt ohne das Flag fehl.
- Ein Experiment-Flag in Build, CI und Docker wäre fehleranfällig. Darum nutzt tollgate das
  API-gleiche Modul `github.com/go-json-experiment/json`.
- Sobald `encoding/json/jsontext` ohne Flag stabil ist, reicht ein Austausch des Imports.
- `jsontext` liefert einen strikten RFC-8259-Tokenizer mit `InputOffset()`. Genau das braucht
  das Byte-Splicing (ADR-0004).
- Verworfen:
  - `tidwall/gjson` und `sjson`: weniger strikt, Pfad-Syntax statt Token-Stream.
  - `encoding/json` mit `json.RawMessage`: gibt keine Offsets, erzwingt Neu-Serialisierung
    der Hüllobjekte.

### Frontend

| Zweck | Wahl | Stand | Verworfen, weil |
|---|---|---|---|
| Framework | Svelte 5 (Runes) | 5.x | Vorgabe |
| Build | Vite, statischer Build nach `web/dist` | 8.x | Vorgabe |
| Sprache | TypeScript, `strict` | – | Vorgabe |
| Paketmanager | pnpm | 10.x | npm ist laut Arbeitsumgebung ausgeschlossen |
| Routing | eigener, kleiner Hash-Router. Detailansichten klappen inline auf, es gibt also kaum Routen | – | SvelteKit: für ein eingebettetes statisches SPA zu viel |
| YAML-Editor | CodeMirror 6, `@codemirror/lang-yaml`, `codemirror-json-schema` (YAML-Modus) | 6.x / 0.8.x | Monaco: zu groß für mobile Nutzung |
| Tests | Vitest, `@testing-library/svelte` | – | – |
| UI-Kit | keines, eigene CSS-Tokens (Farben, Abstände, Typografie, Dark Mode) | – | UI-Kits widersprechen dem ruhigen, Things-3-inspirierten Design und bringen fremde Icons mit |
| Icons | eigene SVG-Icons oder eine MIT-lizenzierte Icon-Bibliothek, keine Things-3-Assets | – | – |

Alle Versionen werden in `go.mod`/`go.sum` bzw. `pnpm-lock.yaml` gepinnt. Updates erfolgen
bewusst über eigene Issues.

## Alternativen

Siehe Spalte „Verworfen, weil“.

Grundsätzlich verworfen wurde ein Web-Framework im Backend (gin, echo). tollgate hat zwei
Arten von Endpunkten:
- einen Proxy, der bytegenau arbeiten muss
- eine kleine JSON-API

Beides deckt die Standardbibliothek vollständig ab. Frameworks erschweren das bytegenaue
Durchreichen eher.

## Konsequenzen

- Das Backend hat etwa sechs direkte Abhängigkeiten, alle ohne cgo. `CGO_ENABLED=0` erzeugt
  ein statisches Binary.
- `go-json-experiment/json` hat keine Release-Tags. Es wird über eine Pseudo-Version gepinnt,
  ein Update ist ein eigenes Issue.
- **#2** legt `go.mod`, `tygo`-Konfiguration und `make generate` an.
- **#4** setzt SQLite und Migrationen um, **#9** das Frontend-Gerüst.

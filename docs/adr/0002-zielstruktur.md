# ADR-0002: Zielstruktur des Repos

- **Status:** Angenommen (Freigabe Niclas, 2026-10-04, PR #26); teilweise abgelöst durch [ADR-0014](0014-cliproxyapi-gepinnt-ohne-kopie.md) (Zeile `third_party/cliproxyapi/`)
- **Datum:** 2026-10-04
- **Issue:** #1

## Kontext

`CLAUDE.md` schlägt eine Zielstruktur vor und verlangt, sie im ADR zu bestätigen oder
begründet zu ändern. Aus ADR-0001, ADR-0004 und ADR-0006 ergeben sich drei Querschnittsthemen,
die mehrere Pakete gemeinsam brauchen:
- bytegenaue JSON-Bearbeitung
- SSE-Verarbeitung
- Schlüsselprüfung

## Entscheidung

Die vorgeschlagene Struktur wird bestätigt und um fünf Einträge ergänzt (markiert mit `+`):

```
tollgate/
  CLAUDE.md                     Projektregeln für künftige Sessions
  Makefile                      make build | make test | make generate
  go.mod                        Modul github.com/niclasedge/tollgate
  cmd/tollgate/                 main, ein einziges tollgate-Binary
  internal/
    pipeline/                   Stage-Interface, Request-Kontext, Trace-IDs, Sitzungsschlüssel
    redact/                     Go-Port des Redactors, Platzhalter, Rehydrator
      testdata/                 txtar-Korpus aus dem Rust-Repo
    compress/                   Regeln pro Befehl und Headroom-Port
      testdata/                 Golden-Files pro Regel
    meter/                      Token-Schätzung, usage-Erfassung, Kosten
    config/                     Schema, YAML-Import/-Export, Versionen, Verschlüsselung
    store/                      SQLite: Traces, Messwerte, Konfigurationsversionen, Sitzungen, Clients
      migrations/               eingebettete SQL-Migrationen
  + jsonsplice/                 Byte-Offsets von JSON-Strings finden und gezielt ersetzen (ADR-0004)
  + sse/                        SSE-Parser und -Writer, ereignisweise (ADR-0006)
  + auth/                       Client-Keys und Admin-Token (ADR-0009)
    upstream/                   HTTP-Client zu CLIProxyAPI inkl. Management-API
    server/                     HTTP-Proxy, Frontend-API, SSE-Live-Feed, eingebettetes Frontend
  web/                          Frontend (Svelte, Vite, TypeScript, pnpm)
    src/lib/types/              von tygo generierte TypeScript-Typen (nicht von Hand pflegen)
  config/schema/                JSON-Schema der Laufzeitkonfiguration (ADR-0008)
+ deploy/cliproxyapi/           Konfigurationsvorlage für CLIProxyAPI, ohne Geheimnisse
  third_party/cliproxyapi/      Kopie als eigenes Go-Modul, LICENSE, PATCHES.md (ADR-0001)
+ third_party/headroom/         nur NOTICE und LICENSE des Headroom-Ports (Code liegt in internal/compress)
  third_party/NOTICE.md         Herkunft und Lizenzen aller übernommenen Teile
  testdata/fixtures/            aufgezeichnete, bereinigte Anfragen (E2E, Dry-Run)
  docs/ARCHITECTURE.md, docs/adr/, docs/CLIENT_SETUP.md
  docker-compose.yml
```

Begründung der Ergänzungen:

- **`internal/jsonsplice`:** Redact, Compress und Meter brauchen dieselbe bytegenaue
  Bearbeitung. Ein eigenes Paket verhindert drei abweichende Implementierungen und lässt sich
  isoliert fuzzen.
- **`internal/sse`:** Der Upstream-Client liest SSE, der Rehydrator verändert es, der
  Live-Feed fürs Frontend schreibt SSE. Ein gemeinsamer, gut getesteter Parser reicht.
- **`internal/auth`:** Die Schlüssellogik (Hashing, Sperren, Rotation, Admin-Token) ist
  sicherheitskritisch und wird getrennt von `server/` getestet.
- **`deploy/cliproxyapi`:** Die CPA-Konfiguration gehört nicht in die Kopie unter
  `third_party/`. Sonst wäre sie ein Patch an fremdem Code.
- **`third_party/headroom`:** Apache-2.0 §4 verlangt, LICENSE und NOTICE weiterzugeben. Der
  portierte Code selbst liegt in `internal/compress`, mit Herkunftsvermerk im Dateikopf.

Weitere Festlegungen:
- **Ein Go-Modul** für tollgate. `third_party/cliproxyapi` ist ein zweites, unabhängiges Modul.
- **`ref/` bleibt lokal** und steht in `.gitignore` (siehe `CLAUDE.md`).
- **`web/dist/`** wird nicht committet. `go build` braucht trotzdem ein Verzeichnis zum
  Einbetten. Ein committeter Platzhalter `web/dist/.gitkeep` plus Build-Tag oder ein Fallback
  auf eine Minimalseite sorgen dafür, dass `go test ./...` ohne Frontend-Build läuft. Details
  regelt #9.

## Alternativen

- **Flache Struktur** (alles in `internal/proxy`): verworfen. Die Stufen müssen einzeln
  testbar und im Frontend einzeln sichtbar sein.
- **Mehrere Binaries** (Proxy und Admin getrennt): verworfen. Der Auftrag verlangt ein
  Binary und ein Deployment-Artefakt.

## Konsequenzen

- **#2** legt genau diese Struktur an.
- **#5** nutzt `jsonsplice` für den Eingang, **#6** nutzt `sse` für das Durchreichen.
- **#8** implementiert `internal/auth`.
- **#3** legt `deploy/cliproxyapi/` an, **#20** legt `third_party/headroom/` an.

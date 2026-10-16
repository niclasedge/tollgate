# Design

## Context

Siehe `proposal.md` zur Motivation. Im Repository bestehen Projektregeln, freigegebene Architektur und ADRs, aber kein Go-Modul, Makefile oder CI-Workflow. Es gibt keine aktiven Specs. Lokal ist Go 1.26.5 installiert. #1 ist geschlossen; #2 kann beginnen.

ADR-0002 legt die Verzeichnisstruktur fest, ADR-0003 die Werkzeugwahl. Die vier lokalen Memory-Dateien bleiben außerhalb des Repos. Der fremde, unversionierte Ordner `logs/` bleibt unangetastet.

## Goals / Non-Goals

**Goals:** Ein kleines, baubares Gerüst, das Folge-Issues ohne weitere Grundsatzentscheidungen erweitern können. Build und Tests laufen ohne das spätere Deployment.

**Non-Goals:** Fachliche Interfaces vorwegnehmen, einen HTTP-Server starten, Drittcode kopieren, Laufzeitabhängigkeiten ohne Verwendung einführen oder Frontend/Embedding umsetzen. Docker Compose und Client-Setup entstehen erst bei der Integration und Abnahme. Die Abo-Verwaltung und ADR-0010 gehören zu #28/#29.

## Decisions

### Modul und Pakete

Ein Modul `github.com/niclasedge/tollgate`, Go 1.26.5 als Ausgangsstand. Die Pakete `pipeline`, `redact`, `compress`, `meter`, `config`, `store`, `jsonsplice`, `sse`, `auth`, `upstream` und `server` erhalten jeweils `doc.go` mit Paketbeschreibung. Das hält die Struktur in Git und Go sichtbar, ohne spätere Schnittstellen zu erfinden. `.gitkeep` allein wäre für Go unsichtbar.

Reine Daten-/Ausgabeverzeichnisse erhalten Platzhalter: `internal/redact/testdata`, `internal/compress/testdata`, `internal/store/migrations`, `web/src/lib/types`, `web/dist`, `config/schema`, `deploy/cliproxyapi`, `third_party/cliproxyapi`, `third_party/headroom` und `testdata/fixtures`. Noch keine SQL-Migrationen, Fremdlizenzen oder Konfigurationsvorlagen. Eine knappe `third_party/NOTICE.md` dokumentiert, dass noch kein Drittcode übernommen ist; die Herkunftsnachweise folgen mit #3/#20.

### Minimaler Programmeinstieg

`cmd/tollgate` verwendet einen `slog.JSONHandler` auf stderr, meldet den Gerüststatus und endet erfolgreich. Ein kleiner Starttest prüft Exit-Code und parsebares JSON. Ein dauerhafter Server würde ungeklärtes Start-, Konfigurations- und Bindungsverhalten aus #6 vorwegnehmen.

### Make-Befehle

- `make build`: `CGO_ENABLED=0 go build` nach `bin/tollgate`.
- `make test`: `go test -race ./...`; cgo bleibt hierfür verfügbar, da der Race Detector auf unterstützten CI-Plattformen eine C-Toolchain benötigt.
- `make generate`: das gepinnte `tygo`-Tool über `go tool tygo` ausführen. Die Konfiguration enthält zunächst keine zu generierenden Pakete. tygo v0.2.21 lädt bei einer leeren Paketliste das Root-Paket und scheitert dort ohne Go-Dateien. Darum erkennt Make die explizite Zeile `packages: []`, prüft das gepinnte Tool mit `--version` und beendet die Generierung als dokumentierten No-op. Bei einer nicht leeren Paketliste läuft `go tool tygo generate`. Keine Dummy-Datentypen einführen. Mit dem ersten API-Typ wird die Paketliste ergänzt.

Frontend-Tests werden in #9 in den gemeinsamen Testbefehl aufgenommen. Jetzt ist kein stilles Überspringen bereits existierender Tests nötig, weil noch kein Frontend vorhanden ist.

### Abhängigkeiten und Generierung

`tygo` wird per `tool`-Direktive auf eine konkrete Version der ADR-0003-Reihe v0.2.x gepinnt; transitive Abhängigkeiten landen in `go.sum`. Laufzeitbibliotheken wie SQLite und JSON-Tokenizer werden erst bei ihrer Verwendung gepinnt. Die Generator-Konfiguration liegt in `tygo.yaml`, Ausgabeziel ist `web/src/lib/types`. Generierte Typen werden später versioniert und von CI auf Aktualität geprüft; sie werden nie von Hand gepflegt.

### CI und Git-Regeln

Ein Workflow für Pull Requests und Pushes auf `main` nutzt einen Ubuntu-Runner mit C-Toolchain für den Race Detector, `actions/setup-go` mit `go-version-file: go.mod` und nur lesenden Repository-Rechten. Checkout und Setup-Actions bei Apply auf verifizierte Commit-SHAs pinnen. Cache anhand von `go.sum`.

CI führt die drei Make-Befehle aus. Nach der Generierung prüfen `git diff --exit-code` sowie eine Prüfung auf nicht ignorierte unversionierte Dateien, dass Moduldateien und generierte Typen aktuell sind. So werden auch neue Ausgaben erkannt. Diese Prüfung läuft in einem frischen CI-Checkout und bezieht den lokalen Ordner `logs/` nicht ein.

`.gitignore` ignoriert `/bin/` und `/web/dist/*`, erlaubt ausdrücklich `/web/dist/.gitkeep` und erhält die bestehende Regel für `ref/`. Der Platzhalter wird noch nicht eingebettet: Dotfiles allein erfüllen ein normales `go:embed`-Muster nicht. Fallback und echtes Frontend-Embedding werden in #9 entschieden.

## Risks / Trade-offs

- Race Detector benötigt cgo → `CGO_ENABLED=0` nur für den Binary-Build setzen, Tests mit C-Toolchain ausführen.
- Leere Pakete beweisen noch keine fachliche Funktion → CI prüft nur Gerüst und Startvertrag; fachliche Tests entstehen mit den jeweiligen Issues.
- Generator ohne API-Typen → Leerlauf explizit unterstützen und dokumentieren, später echte Paketkonfiguration ergänzen.
- Platzhalter könnten fertige Drittintegration suggerieren → Entwicklungsdokumentation und NOTICE erklären den Gerüststatus.

## Migration Plan

Keine bestehende Laufzeit oder Datenbank wird migriert. Gerüst auf einem Issue-Branch implementieren, lokal die drei Make-Befehle prüfen und PR mit `Closes #2` erstellen. Nach grünem CI und Merge den Change separat archivieren. Ein Revert entfernt das Gerüst ohne Datenmigration.

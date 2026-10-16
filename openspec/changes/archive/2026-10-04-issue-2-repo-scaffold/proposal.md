# Proposal

Issue: [#2 Repo-Scaffold: Go-Modul, Struktur, make test, CI](https://github.com/niclasedge/tollgate/issues/2)

Geht zurueck auf GitHub-Issue #2.

## Why

## Warum

Die Architektur ist mit #1 freigegeben, das Repository enthält aber noch kein baubares Go-Gerüst. Ein reproduzierbarer Build, ein gemeinsamer Testbefehl und CI bilden die Grundlage für die Folge-Issues.

## What Changes

## Was ändert sich

- Go-Modul `github.com/niclasedge/tollgate` und ein minimales ausführbares `cmd/tollgate` mit JSON-Logging über `log/slog` anlegen.
- Die Paket- und Verzeichnisstruktur aus ADR-0002 versionierbar vorbereiten, einschließlich `jsonsplice`, `sse`, `auth`, Deployment- und Drittanbieter-Verzeichnissen.
- `make build`, `make test` und `make generate` bereitstellen; Tests laufen mit Race Detector, der Binary-Build ohne cgo.
- `tygo` als gepinntes Go-Tool samt Konfiguration für später generierte TypeScript-Typen vorbereiten.
- GitHub Actions für Tests, statischen Build und Generierungsprüfung anlegen.
- Einen versionierten Platzhalter unter `web/dist` erlauben und generierte Build-Artefakte ignorieren.

## Capabilities

### New Capabilities

- `repo-scaffold`: Baubares Gerüst mit dokumentierten Entwicklungsbefehlen, strukturiertem Logging und automatischer Prüfung in CI.

### Modified Capabilities

Keine; es bestehen noch keine aktiven Specs.

## Impact

- Neue Dateien: `go.mod`, `go.sum`, `Makefile`, `tygo.yaml`, `cmd/tollgate`, Paketdokumentation und Platzhalter gemäß ADR-0002, `.github/workflows/ci.yml`, Entwicklungsdokumentation.
- Änderung: `.gitignore` für Build-Ausgaben und die Ausnahme `web/dist/.gitkeep`.
- Neue Werkzeugabhängigkeit: `github.com/gzuidhof/tygo` gemäß ADR-0003; Laufzeitabhängigkeiten werden erst mit ihren jeweiligen Implementierungen eingeführt.
- Keine Proxy-Routen, Datenbank, Authentifizierung, Frontend-Anwendung oder Abo-Verwaltung. Kein Drittcode wird kopiert. #3, #4 und weitere Folge-Issues bauen auf diesem Gerüst auf.

## Was ausdrücklich nicht

Keine Proxy-Routen, Datenbank, Authentifizierung, Frontend-Anwendung, Abo-Verwaltung oder kopierter Drittcode. Keine Änderungen an ops-lab-Gates oder fremden Changes.

## Berührte Dateien

- `go.mod`
- `go.sum`
- `Makefile`
- `tygo.yaml`
- `.gitignore`
- `.github/workflows/ci.yml`
- `README.md`
- `cmd/tollgate/main.go`
- `cmd/tollgate/main_test.go`
- `cmd/tollgate/contract_test.go`
- `internal/pipeline/doc.go`
- `internal/redact/doc.go`
- `internal/compress/doc.go`
- `internal/meter/doc.go`
- `internal/config/doc.go`
- `internal/store/doc.go`
- `internal/jsonsplice/doc.go`
- `internal/sse/doc.go`
- `internal/auth/doc.go`
- `internal/upstream/doc.go`
- `internal/server/doc.go`
- `internal/redact/testdata/.gitkeep`
- `internal/compress/testdata/.gitkeep`
- `internal/store/migrations/.gitkeep`
- `web/src/lib/types/.gitkeep`
- `web/dist/.gitkeep`
- `config/schema/.gitkeep`
- `deploy/cliproxyapi/.gitkeep`
- `third_party/cliproxyapi/.gitkeep`
- `third_party/headroom/.gitkeep`
- `third_party/NOTICE.md`
- `testdata/fixtures/.gitkeep`

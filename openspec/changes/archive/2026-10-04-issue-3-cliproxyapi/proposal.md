# Proposal

Issue: [#3 CLIProxyAPI einbinden (third_party, NOTICE, docker-compose)](https://github.com/niclasedge/tollgate/issues/3)

## Why

tollgate leitet an CLIProxyAPI weiter (ADR-0001). Dafür fehlen die Kopie des Upstream mit Lizenznachweis, eine Pflichtkonfiguration ohne Geheimnisse, ein Deployment ohne veröffentlichten Port und ein Client, der CLIProxyAPI lokal erreicht und seinen Zustand prüft. Ohne das kann der Proxy (#6) nicht gegen den echten Upstream entwickelt und abgenommen werden.

## What Changes

- CLIProxyAPI @ `8ef43e4df3b216a42493105d31c2873b69191473` als eigenständiges Go-Modul unter `third_party/cliproxyapi` kopieren (MIT, `LICENSE` bleibt), mit `PATCHES.md` (Commit-Hash, begründete Abweichungen) und Eintrag in `third_party/NOTICE.md`.
- Konfigurationsvorlage `deploy/cliproxyapi/config.yaml` mit den Pflichtwerten aus ADR-0001 und Platzhaltern statt Geheimnissen; `render-config.sh` füllt sie aus Secret-Dateien. `Dockerfile` baut CLIProxyAPI mit `CGO_ENABLED=0`.
- `docker-compose.yml` mit dem Dienst `cliproxyapi`: kein veröffentlichter Port, Secrets aus Dateien, auth-dir als Volume.
- `internal/upstream`: minimaler Client (`New`, `Health`), der nur Loopback-Ziele akzeptiert. Ein Smoke-Test startet das echte Binary mit der gerenderten Vorlage.

## Capabilities

### New Capabilities

- `cliproxyapi-integration`: Eingebundene, lokal betriebene CLIProxyAPI-Instanz mit nachweisbarer Herkunft, Pflichtkonfiguration und Health-Check aus tollgate.

### Modified Capabilities

Keine.

## Impact

- Neue Dateien unter `third_party/cliproxyapi/` (rund 1.800 Dateien, eigenes `go.mod`), `third_party/NOTICE.md`, `deploy/cliproxyapi/`, `docker-compose.yml`, `internal/upstream/`.
- Root-`go.mod`/`go.sum`: unverändert; `go list ./...` im Root schließt die Kopie nicht ein. Keine neue Laufzeitabhängigkeit.
- Offen für andere Changes: `/secrets/` in die `.gitignore` (Compose-Secrets), tollgate-Dienst in `docker-compose.yml` (#11), Management-Abfragen (aktiver Account, Baseline-Wächter) in #10.

## Was ausdrücklich nicht

Kein Proxy und keine Header-Regeln (#6), keine Abfrage des aktiven Accounts oder Baseline-Wächter (#10), kein tollgate-Dienst und keine Tailnet-Topologie in Compose (#11), keine Abo-Verwaltung (#28/#29), keine Codeänderung an CLIProxyAPI, keine echten Zugangsdaten und kein Login-Ablauf.

## Berührte Dateien

- `third_party/cliproxyapi/**` (Kopie, `LICENSE`, `PATCHES.md`; `.gitkeep` bleibt)
- `third_party/NOTICE.md`
- `deploy/cliproxyapi/config.yaml`
- `deploy/cliproxyapi/render-config.sh`
- `deploy/cliproxyapi/Dockerfile`
- `docker-compose.yml`
- `internal/upstream/doc.go`
- `internal/upstream/client.go`
- `internal/upstream/client_test.go`
- `internal/upstream/smoke_test.go`
- `openspec/changes/issue-3-cliproxyapi/**`

# issue-107-docker-build-kontext

Issue: [#107 Docker: tollgate-Image baut nicht (fixtures.go, config/, testdata fehlen im Kontext)](https://github.com/niclasedge/tollgate/issues/107)

## Warum

Geht zurück auf GitHub-Issue #107, gefunden in #59. `docker build -f deploy/tollgate/Dockerfile .`
scheitert auf `main` im Schritt `go build`: `no required module provides package
github.com/niclasedge/tollgate` bzw. `.../config/schema`. Seit #15 importiert `cmd/tollgate`
das Root-Paket (`fixtures.go` mit `go:embed testdata/fixtures/*.json`) und `config/schema`.
Das Dockerfile kopiert beides nicht, und `.dockerignore` schließt `testdata` aus. Damit bauen
`just setup` und `just smoke` kein Image. Kein Test hat die Lücke bemerkt, weil die Tests
nur den Build im Repo prüfen, nicht im Docker-Kontext.

## Was ändert sich

- `deploy/tollgate/Dockerfile` kopiert in der Build-Stage zusätzlich `fixtures.go`,
  `config/` und `testdata/fixtures/`.
- `.dockerignore` schließt `testdata` weiter aus und gibt nur `testdata/fixtures` frei.
- Ein Go-Test (`deploy/buildcontext_test.go`) ermittelt mit `go list -deps -json ./cmd/tollgate`
  alle Pakete dieses Moduls samt Go- und Embed-Dateien und prüft, dass jede Datei von einer
  `COPY`-Zeile der Build-Stage an ihren Pfad unter `WORKDIR` gelegt und nicht von
  `.dockerignore` ausgeschlossen wird.

## Was ausdrücklich nicht

- Keine Änderung an den Basis-Image-Digests (#59) und am CPA-Dockerfile.
- Keine weiteren Verzeichnisse im Build-Kontext; Secrets, `ref/`, `docs/` usw. bleiben draußen.
- Keine neue Abhängigkeit für die `.dockerignore`-Auswertung; der Test deckt die im Repo
  genutzte Syntax ab und lehnt `**` ausdrücklich ab.

## Berührte Dateien

- `deploy/tollgate/Dockerfile`, `.dockerignore`
- `deploy/buildcontext_test.go` (neu)
- `openspec/changes/issue-107-docker-build-kontext/**`

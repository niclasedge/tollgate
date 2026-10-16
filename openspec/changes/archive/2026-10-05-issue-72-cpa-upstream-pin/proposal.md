# issue-72-cpa-upstream-pin

Issue: [#72 CLIProxyAPI nicht mehr ins Repo kopieren, im Container-Build gepinnt holen](https://github.com/niclasedge/tollgate/issues/72)

## Warum

Geht zurueck auf GitHub-Issue #72.

`third_party/cliproxyapi/` ist eine vollständige Kopie von CLIProxyAPI: rund 32 MB und
1.785 Dateien, ohne einen einzigen Codepatch. Die Kopie stammt aus der ursprünglichen
Vorgabe in AGENTS.md, umgesetzt in ADR-0001 und #3. Sie bläht jeden Checkout auf, macht
Upstream-Updates zu einem großen Dateitausch und ist im künftig öffentlichen Repo fremder
Code ohne Mehrwert. ADR-0001 nennt einen Build ohne Kopie bereits als Rückfalloption.
Das Upstream-Modul `github.com/router-for-me/CLIProxyAPI/v8` hat keine `replace`-Direktiven
und lässt sich deshalb mit `go install …@COMMIT` reproduzierbar bauen; die Prüfsumme kommt
aus sum.golang.org.

## Was aendert sich

- `deploy/cliproxyapi/Dockerfile` baut CLIProxyAPI mit `CGO_ENABLED=0` per
  `go install github.com/router-for-me/CLIProxyAPI/v8/cmd/server@${CPA_VERSION}`. Der Pin
  steht genau einmal als `ARG CPA_VERSION=…`; der Named Context `cpa-src` entfällt.
- `make cpa` liest denselben Pin aus dem Dockerfile und baut `bin/cli-proxy-api` für den
  Smoke-Test (`TOLLGATE_CPA_BIN`). CI baut am Pin und lässt den Smoke-Test laufen.
- `deploy/cliproxyapi/UPSTREAM.md` nennt Commit, Lizenz, Update-Ablauf und wie man die
  lokale Referenz `ref/cliproxyapi` (gitignored, Klon auf den Pin) anlegt.
- `third_party/cliproxyapi/` wird gelöscht; `third_party/NOTICE.md` behält Herkunft, Lizenz
  und Copyright.
- ADR-0014 löst ADR-0001 Punkt 2 und 3 ab; AGENTS.md (Referenzen, Zielstruktur),
  ARCHITECTURE.md und der ADR-Index werden nachgezogen.

## Was ausdruecklich nicht

- Keine Bereinigung der Git-History; die Kopie bleibt dort. Die Entscheidung dazu fällt
  vor dem Public-Schalten.
- Kein Wechsel der CPA-Version: der Pin bleibt `8ef43e4df3b216a42493105d31c2873b69191473`.
- Keine Änderung an der CPA-Konfiguration, an `render-config.sh` oder am tollgate-Image.
- `ref/cliproxyapi` wird nur dokumentiert, nicht angelegt.

## Beruehrte Dateien

- `deploy/cliproxyapi/Dockerfile`, `deploy/cliproxyapi/UPSTREAM.md`
- `docker-compose.yml`
- `Makefile`, `.github/workflows/ci.yml`
- `internal/upstream/pin_test.go`, `internal/upstream/smoke_test.go` (Kommentar)
- `cmd/tollgate/main_test.go`
- `third_party/cliproxyapi/**` (gelöscht), `third_party/NOTICE.md`
- `AGENTS.md`, `docs/ARCHITECTURE.md`, `docs/adr/0014-cliproxyapi-gepinnt-ohne-kopie.md`,
  `docs/adr/0001-cliproxyapi-als-eigener-prozess.md`, `docs/adr/0002-zielstruktur.md`,
  `docs/adr/README.md`
- `openspec/changes/issue-72-cpa-upstream-pin/**`

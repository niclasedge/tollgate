# ADR-0014: CLIProxyAPI gepinnt aus dem Upstream statt als Kopie

- **Status:** Angenommen (Freigabe Niclas, 2026-10-04, Issue #72)
- **Datum:** 2026-10-04
- **Issue:** #72
- **Löst ab:** ADR-0001 Punkt 2 und 3, in ADR-0002 die Zeile `third_party/cliproxyapi/`

## Kontext

ADR-0001 legte CLIProxyAPI (CPA) als Kopie unter `third_party/cliproxyapi/` ab, weil der
Auftrag das verlangte. Nach #3 lag dort der vollständige Upstream-Stand, rund 32 MB und
1.785 Dateien, ohne einen einzigen Codepatch (`PATCHES.md` nannte nur zwei entfernte
Dateigruppen). Die Kopie hat keine Funktion, die ein gepinnter Build nicht auch erfüllt:

- tollgate importiert CPA nie; es spricht nur per HTTP mit dem Prozess (ADR-0001 Punkt 1).
- Das Upstream-Modul `github.com/router-for-me/CLIProxyAPI/v8` hat keine `replace`- oder
  `exclude`-Direktiven. `go install …/cmd/server@<commit>` baut deshalb exakt diesen Stand,
  und sum.golang.org prüft die Prüfsumme.
- Der Build ohne cgo funktioniert auch so: nur der Plugin-Loader braucht cgo, und der
  Upstream bringt dafür einen `!cgo`-Stub mit.
- Das Repo soll public werden; fremder Code ohne Patch bläht es nur auf.

ADR-0001 nannte ein gepinntes Image beziehungsweise einen Build ohne Kopie bereits als
Rückfalloption.

## Entscheidung

1. **Kein CPA-Code im Repo.** `third_party/cliproxyapi/` entfällt. `third_party/NOTICE.md`
   behält Quelle, Commit, Lizenz und Copyright.
2. **Pin an genau einer Stelle.** `deploy/cliproxyapi/Dockerfile` trägt
   `ARG CPA_VERSION=<commit>` und baut mit
   `CGO_ENABLED=0 go install github.com/router-for-me/CLIProxyAPI/v8/cmd/server@${CPA_VERSION}`.
   Das Binary heißt danach `server` und wird in `cli-proxy-api` umbenannt. Die `LICENSE` des
   Moduls liegt im Image unter `/usr/share/doc/cli-proxy-api/LICENSE`. `make cpa` liest den
   Pin aus dem Dockerfile und baut `bin/cli-proxy-api` für den Smoke-Test.
   `deploy/cliproxyapi/UPSTREAM.md` dokumentiert Commit, Lizenz und Update-Ablauf;
   `internal/upstream/pin_test.go` prüft, dass Dokumentation und Pin übereinstimmen.
3. **Lokale Referenz.** Zum Lesen neuer Versionen liegt der Quellcode wie Redactor und Headroom
   unter `ref/cliproxyapi` (Klon auf den Pin, gitignored). `UPSTREAM.md` beschreibt das Anlegen.
4. **Update-Ablauf** (ersetzt ADR-0001 Punkt 3): neuen Commit in `ref/cliproxyapi` lesen,
   `config.example.yaml` gegen die Vorlage vergleichen, `ARG CPA_VERSION` und die Doku
   nachziehen, `make cpa` mit Smoke-Test, `docker compose build cliproxyapi`, `just smoke`.
   CI baut am Pin und lässt den Smoke-Test bei jedem PR laufen.

Ziel bleibt: keine Patches. Wird doch einer nötig, braucht er ein neues ADR, weil
`go install` keinen gepatchten Stand bauen kann.

## Alternativen

- **Kopie behalten:** verworfen, siehe Kontext.
- **Fremdes Image `eceasy/cli-proxy-api:<tag>`:** verworfen. Ein Tag ist veränderlich, ein
  Digest nicht nachvollziehbar auf einen Commit abbildbar, und das Image enthält einen
  cgo-Build mit anderer Basis. Der eigene Build bleibt ein kleiner Schritt.
- **Git-Submodul:** verworfen. Es holt denselben Code bei jedem rekursiven Checkout und
  verlangt Submodul-Pflege in CI, ohne Vorteil gegenüber dem Modul-Proxy.

## Konsequenzen

- Der Container-Build und `make cpa` brauchen Netzzugang zum Go-Modul-Proxy, ein Offline-Build
  aus dem Repo allein ist nicht mehr möglich.
- Ein Upstream-Update ist eine Ein-Zeilen-Änderung plus Doku statt eines Tauschs von
  1.785 Dateien.
- Die Kopie bleibt in der Git-History. Ob sie vor dem Public-Schalten entfernt wird, ist
  offen und nicht Teil dieses ADR.
- AGENTS.md (Referenzen, Zielstruktur), `docs/ARCHITECTURE.md` und
  `openspec/specs/cliproxyapi-integration` werden nachgezogen.

## Quellen

CLIProxyAPI @ `8ef43e4df3b216a42493105d31c2873b69191473`
(`v8.0.14-0.20261004053141-8ef43e4df3b2`): `go.mod` (keine `replace`/`exclude`),
`cmd/server`, `LICENSE` (MIT).

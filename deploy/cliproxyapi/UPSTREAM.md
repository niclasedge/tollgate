# CLIProxyAPI: gepinnter Upstream

- **Upstream:** https://github.com/router-for-me/CLIProxyAPI
- **Modul:** `github.com/router-for-me/CLIProxyAPI/v8`, Paket `cmd/server`
- **Commit:** `6db7d4b839d86c5b24e17bbe73bcd4db18ef1b1a`
  (Tag `v8.0.63`)
- **Lizenz:** MIT
  - Copyright (c) 2025-2005.9 Luis Pater
  - Copyright (c) 2025.9-present Router-For.ME
- **Einbindung:** eigener Prozess, nie von tollgate importiert
  ([ADR-0001](../../docs/adr/0001-cliproxyapi-als-eigener-prozess.md)). Der Code liegt nicht
  im Repo, sondern wird beim Bauen aus dem Go-Modul-Proxy geholt
  ([ADR-0014](../../docs/adr/0014-cliproxyapi-gepinnt-ohne-kopie.md)). Die begrenzte Refresh-Barriere aus [ADR-0016](../../docs/adr/0016-cpa-refresh-barriere.md) wird nur im temporären Build angewendet.

Herkunft und Lizenz stehen zusätzlich in [`third_party/NOTICE.md`](../../third_party/NOTICE.md).

## Wo der Pin steht

Technisch nur an einer Stelle: `ARG CPA_VERSION=…` in [`Dockerfile`](Dockerfile). Das
Image lädt damit die Quellen per `go mod download`, prüft die dokumentierte Refresh-Barriere
und baut das Binary mit `CGO_ENABLED=0`; nur der Plugin-Loader braucht cgo, und dafür hat der Upstream einen
`!cgo`-Stub. Die `LICENSE` des Moduls kopiert der Build nach
`/usr/share/doc/cli-proxy-api/LICENSE` im Image. `make cpa` liest denselben Wert aus dem Dockerfile und baut `bin/cli-proxy-api` auf demselben Weg wie das Image.
Der Commit in dieser Datei und in `NOTICE.md` ist Dokumentation; `internal/upstream/pin_test.go`
prüft, dass alle drei übereinstimmen.

Seit v8.0.55 hat das Upstream-`go.mod` genau ein `replace`: `github.com/MoeForge/moego` →
`github.com/router-for-me/moego` (MIT, Fork des Upstream-Teams). `go install …@<commit>`
lehnt das ab; Image und `make cpa` bauen deshalb in einer temporären Kopie des Moduls, in der
das `replace` greift ([ADR-0022](../../docs/adr/0022-cpa-v8-0-63.md)). Die Prüfsummen
kommen aus sum.golang.org und dem Upstream-`go.sum`.

## Lokale Referenz

Zum Lesen und Vergleichen neuer Versionen liegt der Quellcode lokal unter `ref/cliproxyapi`,
wie `ref/hook-redactor` und `ref/headroom`. `ref/` steht in `.gitignore` und wird nie
committet.

```sh
git clone https://github.com/router-for-me/CLIProxyAPI.git ref/cliproxyapi
git -C ref/cliproxyapi checkout --detach 6db7d4b839d86c5b24e17bbe73bcd4db18ef1b1a
```

Liegt der Klon schon an anderer Stelle, reicht ein Symlink:
`ln -s /pfad/zum/klon ref/cliproxyapi`.

## Update-Ablauf

1. Neuen Commit wählen und in `ref/cliproxyapi` auschecken. Prüfen, dass `go.mod` keine
   `replace`-Direktiven außer dem für `moego` hat (Ziel, Lizenz, `go.sum`-Eintrag) und die
   `LICENSE` unverändert MIT ist.
2. Die Änderungen seit dem alten Pin lesen, mindestens die Claude-Erkennung, das Cloaking
   und die Management-API (Quellen in ADR-0001).
3. `config.example.yaml` gegen [`config.yaml`](config.yaml) vergleichen. Neue ausgehende
   Ziele und das Flag `-local-model` prüfen; der Egress-Filter in
   [`entrypoint.sh`](entrypoint.sh) lässt nur Anthropic durch
   ([ADR-0017](../../docs/adr/0017-cpa-egress-nur-anthropic.md)). Die Anthropic-Bereiche
   dort gegen die veröffentlichte Liste („IP addresses“ in der Anthropic-Doku) halten.
4. `ARG CPA_VERSION` im Dockerfile setzen und den Commit hier sowie in `third_party/NOTICE.md`
   nachziehen.
5. `make cpa` und
   `TOLLGATE_CPA_BIN=$PWD/bin/cli-proxy-api go test -race -count=1 -run Smoke ./internal/upstream/`.
6. `docker compose build cliproxyapi` und `just smoke`.

## Refresh-Barriere (#77)

`bridge/apply.sh` ergänzt ausschließlich im Build den authentifizierten Pause-Endpunkt.
Begründung, Update-Regeln und Tests: [PATCHES.md](../../third_party/cpa-refresh-barrier/PATCHES.md).
Der Image-Build führt den Race-Regressionstest am Pin aus. Bei Updates beide
Einfügestellen, Credential-Persistenz und Mutex-Laufzeit prüfen.

## Header-Baseline und kv-store (#4, ADR-0022)

Ab v8.0.63 hebt CPA die Claude-Code-Baseline bei jedem Release an; ein älterer Client gilt
dann nicht mehr als Claude Code und bekommt Betas und Header umgeschrieben.
[`config.yaml`](config.yaml) pinnt deshalb `upstream.claude.header-defaults` auf
`claude-cli/2.1.280`. Bei Updates prüfen, ob `helps/claude_device_profile.go` die
Schwelle oder die Header-Logik ändert. `server.kv-store.disabled: true` hält die
bbolt-Datei aus dem auth-dir. Neue Body-Eingriffe des Claude-Executors (zuletzt
`thinking.block_binding`) und neue ausgehende Ziele (zuletzt der xAI-Check gegen
`registry.npmjs.org`, vom Egress-Filter verworfen) im ADR nachtragen.

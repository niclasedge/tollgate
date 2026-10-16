# CLIProxyAPI: gepinnter Upstream

- **Upstream:** https://github.com/router-for-me/CLIProxyAPI
- **Modul:** `github.com/router-for-me/CLIProxyAPI/v8`, Paket `cmd/server`
- **Commit:** `8ef43e4df3b216a42493105d31c2873b69191473`
  (Pseudo-Version `v8.0.14-0.20261004053141-8ef43e4df3b2`)
- **Lizenz:** MIT
  - Copyright (c) 2025-2005.9 Luis Pater
  - Copyright (c) 2025.9-present Router-For.ME
- **Einbindung:** eigener Prozess, nie von tollgate importiert
  ([ADR-0001](../../docs/adr/0001-cliproxyapi-als-eigener-prozess.md)). Der Code liegt nicht
  im Repo, sondern wird beim Bauen aus dem Go-Modul-Proxy geholt
  ([ADR-0014](../../docs/adr/0014-cliproxyapi-gepinnt-ohne-kopie.md)). Die begrenzte Refresh-Barriere aus [ADR-0016](../../docs/adr/0016-cpa-refresh-barriere.md) wird nur im temporären Build angewendet, ebenso das Katalog-Overlay aus [ADR-0020](../../docs/adr/0020-cpa-katalog-overlay.md).

Herkunft und Lizenz stehen zusätzlich in [`third_party/NOTICE.md`](../../third_party/NOTICE.md).

## Wo der Pin steht

Technisch nur an einer Stelle: `ARG CPA_VERSION=…` in [`Dockerfile`](Dockerfile). Das
Image lädt damit die Quellen per `go mod download`, prüft die dokumentierte Refresh-Barriere
und baut das Binary mit `CGO_ENABLED=0`; nur der Plugin-Loader braucht cgo, und dafür hat der Upstream einen
`!cgo`-Stub. Die `LICENSE` des Moduls kopiert der Build nach
`/usr/share/doc/cli-proxy-api/LICENSE` im Image. `make cpa` liest denselben Wert aus dem Dockerfile und baut `bin/cli-proxy-api`.
Der Commit in dieser Datei und in `NOTICE.md` ist Dokumentation; `internal/upstream/pin_test.go`
prüft, dass alle drei übereinstimmen.

`go install` funktioniert, weil das Upstream-`go.mod` keine `replace`- oder
`exclude`-Direktiven hat. Die Prüfsumme des Moduls kommt aus sum.golang.org.

## Lokale Referenz

Zum Lesen und Vergleichen neuer Versionen liegt der Quellcode lokal unter `ref/cliproxyapi`,
wie `ref/hook-redactor` und `ref/headroom`. `ref/` steht in `.gitignore` und wird nie
committet.

```sh
git clone https://github.com/router-for-me/CLIProxyAPI.git ref/cliproxyapi
git -C ref/cliproxyapi checkout --detach 8ef43e4df3b216a42493105d31c2873b69191473
```

Liegt der Klon schon an anderer Stelle, reicht ein Symlink:
`ln -s /pfad/zum/klon ref/cliproxyapi`.

## Update-Ablauf

1. Neuen Commit wählen und in `ref/cliproxyapi` auschecken. Prüfen, dass `go.mod` weiterhin
   keine `replace`-Direktiven hat und die `LICENSE` unverändert MIT ist.
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

## Katalog-Overlay (#137)

`bridge/models_overlay.go.txt` ergänzt im Build `claude-haiku-5-5`, das der eingebettete
Katalog am Pin nicht kennt. Begründung: [PATCHES.md](../../third_party/cpa-model-overlay/PATCHES.md).
Bei Updates prüfen, ob `internal/registry/models/models.json` das Modell inzwischen
enthält. Wenn ja, Overlay, Test, `apply.sh`-Zeilen und Dockerfile-Test entfernen.

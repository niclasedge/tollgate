# issue-75-cpa-egress

Issue: [#75 CPA-Container: Egress nur zu Anthropic (iptables im Netz-Namespace) + -local-model](https://github.com/niclasedge/tollgate/issues/75)

## Warum

Geht zurück auf GitHub-Issue #75 (Security-Review von CLIProxyAPI am Pin `8ef43e4`).
CLIProxyAPI baut ohne Not Verbindungen zu Dritten auf: Modellkatalog-Updater zu GitHub und
models.router-for.me (ändern die Model-Registry zur Laufzeit), einen nicht abschaltbaren
Antigravity-Updater zu einem fremden Cloud-Run-Dienst und beim Login einen Public-IP-Lookup.
`requests.proxy-url` deckt diese Clients nicht ab. tollgate teilt den Netz-Namespace mit
CLIProxyAPI und soll ebenfalls nur Anthropic erreichen.

## Was ändert sich

- Neuer Entrypoint `deploy/cliproxyapi/entrypoint.sh`: setzt als root für IPv4 und IPv6
  `OUTPUT DROP` und erlaubt nur `lo`, `ESTABLISHED,RELATED`, `tcp/443` zu `160.79.104.0/23`
  bzw. `2607:6bc0::/48`, DNS nur für uid 0 (Docker-Forwarder) und ICMPv6-Nachbarerkennung.
  Fail-closed: schlägt eine Regel fehl, startet CLIProxyAPI nicht.
- Danach `setpriv` auf `cpa` (uid 10001) ohne Capabilities; CLIProxyAPI startet immer mit
  `-local-model`.
- Dockerfile installiert `iptables`, setzt den Entrypoint und kein `USER` mehr.
- `docker-compose.yml`: `cap_add: [NET_ADMIN]` nur für `cliproxyapi` und `cpa-login`;
  `cpa-login` nutzt denselben Entrypoint mit `-claude-login -no-browser`.
- `deploy/smoke.sh` prüft Egress (Anthropic erreichbar, fremder Host gesperrt) und dass das
  Image ohne `NET_ADMIN` nicht startet.
- ADR-0017, Index, `docs/DEPLOY.md` und `UPSTREAM.md` (Update-Ablauf) nachgezogen.

## Was ausdrücklich nicht

- Kein CPA-Patch, kein Wechsel des Pins; Ingress bleibt wie in ADR-0013.
- Keine DOCKER-USER-Regeln auf dem Host, nur als optionaler Hinweis in `docs/DEPLOY.md`.
- Keine Änderung an tollgate-Code, `web/` oder `internal/server/`.

## Berührte Dateien

- `deploy/cliproxyapi/entrypoint.sh` (neu), `deploy/cliproxyapi/Dockerfile`,
  `deploy/cliproxyapi/UPSTREAM.md`
- `docker-compose.yml`, `deploy/smoke.sh`
- `deploy/cliproxyapi/entrypoint_test.go` (neu)
- `docs/adr/0017-cpa-egress-nur-anthropic.md` (neu), `docs/adr/README.md`, `docs/DEPLOY.md`
- `openspec/changes/issue-75-cpa-egress/**`

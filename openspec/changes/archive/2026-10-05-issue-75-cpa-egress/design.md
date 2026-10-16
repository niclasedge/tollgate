# Design — issue-75-cpa-egress

Entscheidung und Alternativen: [ADR-0017](../../../docs/adr/0017-cpa-egress-nur-anthropic.md).

## Ablauf im Container

1. Docker startet `cpa-entrypoint` als root mit `NET_ADMIN` (Compose).
2. Backend-Wahl je Familie: `iptables`/`ip6tables` (nft), sonst `*-legacy`. Gewählt wird das
   erste Backend, bei dem `-S OUTPUT` gelingt; keines → Exit 1.
3. Regeln je Familie, jede Regel einzeln geprüft (`run … || exit 1`):
   `-P OUTPUT DROP`, `-F OUTPUT`, `-o lo`, `ESTABLISHED,RELATED`, `tcp/443` zum
   Anthropic-Bereich, `udp|tcp/53 --uid-owner 0`, bei IPv6 zusätzlich ICMPv6 133/135/136.
   Die Policy steht vor dem Leeren, damit es nie ein offenes Zeitfenster gibt.
4. `exec setpriv --reuid=cpa --regid=cpa --init-groups --no-new-privs --bounding-set=-all
   --inh-caps=-all -- sh -c 'render-config.sh … && exec cli-proxy-api -config … -local-model "$@"'`.
   Zusätzliche Argumente (Login) hängen hinter `-local-model`.

Die Regeln liegen im Namespace und gelten damit auch für tollgate
(`network_mode: service:cliproxyapi`). Startet `cliproxyapi` neu, setzt der Entrypoint sie
idempotent neu (`-F`).

## Warum DNS nur für uid 0

Docker beantwortet DNS über `127.0.0.11` (über `lo` erlaubt). Auf Linux-Hosts mit
Nicht-Loopback-Upstream öffnet dockerd die Upstream-Sockets im Container-Namespace; sie
gehören uid 0. Unter OrbStack/Docker Desktop leitet der Host weiter, die Regel greift dort
nicht (Zähler 0), schadet aber nicht. CPA und tollgate laufen ohne root.

## Tests

- `deploy/cliproxyapi/entrypoint_test.go` führt den Entrypoint mit Attrappen in `PATH` aus:
  Regelreihenfolge v4/v6, Fail-closed bei Regel- oder Backend-Fehler, Legacy-Fallback,
  Rechteabgabe, `-local-model` und Login-Argumente. Außerdem Compose (`NET_ADMIN` nur für
  `cliproxyapi`/`cpa-login`, kein Entrypoint-Override im Login-Dienst) und Dockerfile.
- `deploy/smoke.sh` prüft im echten Namespace Anthropic erreichbar, fremden Host gesperrt,
  und den Abbruch ohne `NET_ADMIN`.

# ADR-0017: Egress des CPA-Namespaces nur zu Anthropic

- **Status:** Angenommen
- **Datum:** 2026-10-05
- **Issue:** #75
- **Ergänzt:** [ADR-0013](0013-veroeffentlichung-auf-host-tailnet-ip.md) (Ingress bleibt unverändert)

## Kontext

Das Security-Review von CLIProxyAPI (CPA) am Pin `8ef43e4` ergab: OAuth-Token und
Request-Inhalte gehen nur an Anthropic (`api.anthropic.com`, `platform.claude.com`). CPA
baut aber standardmäßig weitere Verbindungen auf, die tollgate nicht braucht:

- Modellkatalog-Updater (models.json, codex, devin) beim Start und alle 3 h zu
  raw.githubusercontent.com bzw. models.router-for.me. Der entfernte Katalog ändert die
  Model-Registry zur Laufzeit. Abschaltbar nur per Flag `-local-model`
  (`cmd/server/main.go:162`).
- Antigravity-Versions-Updater zu einem Cloud-Run-Dienst eines Dritten, alle 3 h, **nicht
  abschaltbar** (`internal/misc/antigravity_version.go:32`).
- Beim Login mit `-no-browser` ein Public-IP-Lookup über ipify, ifconfig.me, icanhazip und
  ipinfo (`internal/util/ssh_helper.go:18-22`).

`requests.proxy-url` deckt diese Clients nicht ab, sie nutzen `http.DefaultTransport`. Ein
Proxy allein reicht also nicht; die Sperre muss im Netz liegen. tollgate selbst spricht nur
mit CPA auf `127.0.0.1` sowie mit `api.anthropic.com` und `platform.claude.com`. Beide
Namen und `claude.ai` lösen in den von Anthropic veröffentlichten Bereich `160.79.104.0/23`
bzw. `2607:6bc0::/48` auf. tollgate läuft im Netz-Namespace von `cliproxyapi` (ADR-0013)
und erbt dessen Regeln.

## Entscheidung

1. **Paketfilter im Namespace.** Ein neuer Entrypoint
   (`deploy/cliproxyapi/entrypoint.sh`) startet als root und setzt in der Tabelle `filter`
   für IPv4 (`iptables`) und IPv6 (`ip6tables`) die Kette `OUTPUT`:
   - Policy `DROP`, zuerst gesetzt, danach die Kette geleert;
   - erlaubt: Ausgang über `lo`, `ESTABLISHED,RELATED`, `tcp/443` zu `160.79.104.0/23`
     bzw. `2607:6bc0::/48`;
   - erlaubt: `udp/53` und `tcp/53` nur für Sockets von uid 0. Das ist der eingebettete
     DNS-Forwarder von Docker, der seine Upstream-Abfragen auf Linux-Hosts mit
     Nicht-Loopback-Resolvern im Container-Namespace stellt. CPA (uid 10001) und tollgate
     (uid 10002) laufen nie als root und erreichen fremde DNS-Server nicht direkt;
   - nur IPv6: ICMPv6 Router Solicitation, Neighbor Solicitation und Neighbor
     Advertisement, damit IPv6 auf Netzen mit IPv6 funktioniert.
   Die Tabelle `nat` (DNS-Regeln von Docker) bleibt unberührt. Backend: `iptables` (nft),
   ist das nicht nutzbar, `iptables-legacy`. Die Auswahl prüft vorab `-S OUTPUT`, damit nie
   zwei Backends halb gesetzt werden.
2. **Fail-closed.** Schlägt eine Regel fehl oder ist kein Backend nutzbar, endet der
   Entrypoint mit Exit 1, bevor die Konfiguration gerendert oder CPA gestartet wird. Ohne
   CPA wird `cliproxyapi` nicht healthy, und tollgate startet nicht (`depends_on`).
3. **Rechte abgeben.** Danach wechselt der Entrypoint mit `setpriv` auf den Benutzer `cpa`
   (uid 10001), ohne Bounding- und Inheritable-Capabilities und mit `no_new_privs`. Das
   Image setzt dafür kein `USER` mehr; der Entrypoint ist die einzige Stelle, die als root
   läuft. Compose gibt `cliproxyapi` und `cpa-login` zusätzlich nur `NET_ADMIN`; tollgate
   bekommt keine zusätzlichen Capabilities und kann die Regeln nicht ändern.
4. **`-local-model`.** Der Entrypoint startet CPA immer mit `-local-model`; die
   eingebettete Modellliste reicht für Claude. Der Login-Dienst nutzt denselben Entrypoint
   mit `-claude-login -no-browser` als Argumente statt eines eigenen Entrypoints.
5. **Prüfung.** Ein Go-Test führt den Entrypoint mit Attrappen für `iptables`, `setpriv`
   und CPA aus (Reihenfolge, Fail-closed, Backend-Wahl, Rechteabgabe, `-local-model`) und
   prüft Compose und Dockerfile. `just smoke` prüft im laufenden Stack, dass
   `api.anthropic.com:443` erreichbar und ein fremder Host gesperrt ist, und dass das
   Image ohne `NET_ADMIN` nicht startet.

Der CPA-Quellcode bleibt dafür unverändert; die Regeln stehen allein im Entrypoint.

## Alternativen

- **Egress-Proxy-Sidecar mit `internal: true`:** verworfen. Ein internes Netz bricht die
  veröffentlichten Ports aus ADR-0013, und die Updater mit `DefaultTransport` ignorieren
  einen konfigurierten Proxy ohnehin nur teilweise.
- **DNS-Allowlist:** verworfen. Sie setzt nichts durch; jede direkte IP-Verbindung
  umgeht sie.
- **Feste `extra_hosts` und DNS ganz sperren:** verworfen. Das sperrte zwar auch
  DNS-Exfiltration, bricht aber, sobald Anthropic innerhalb des Bereichs eine andere
  Adresse ausliefert.
- **DOCKER-USER-Regeln auf dem Host:** nur als optionale zweite Schicht für Linux-Hosts
  (siehe `docs/DEPLOY.md`). Unter Docker Desktop und OrbStack gibt es keinen Zugriff auf
  die Host-Ketten; die Regel im Namespace gilt überall.
- **CPA patchen, um die Updater abzuschalten:** verworfen. Ein Patch bräuchte ein eigenes
  ADR; der Paketfilter deckt auch künftige neue Ziele ab.

## Konsequenzen

- Der Antigravity-Updater und der IP-Lookup beim Login laufen ins Leere (Timeouts im
  Log). Der Login zeigt die Adresse dann nicht an, funktioniert aber.
- Ändert Anthropic seine veröffentlichten Bereiche, sperrt der Filter Anthropic selbst.
  Die Bereiche stehen im Entrypoint und werden beim CPA-Update mitgeprüft
  (`deploy/cliproxyapi/UPSTREAM.md`).
- Der Host muss `nf_tables` oder `ip_tables` für IPv4 und IPv6 sowie `conntrack` und
  `owner` bereitstellen. Fehlt etwas, startet der Stack nicht (fail-closed).
- CPA und tollgate lösen Namen nur über den eingebetteten DNS von Docker (`127.0.0.11`)
  auf, also in einem benutzerdefinierten Netz wie dem Compose-Standardnetz. Im
  Default-Bridge-Netz (`docker run` ohne `--network`) fragen sie den Host-Resolver direkt
  und werden gesperrt.
- DNS-Anfragen über Docker bleiben als schmaler Seitenkanal; CPA kann beliebige Namen
  auflösen lassen, aber keine Verbindung außerhalb von Anthropic aufbauen.
- Die Hilfsläufe in `deploy/setup.sh` mit `--entrypoint sh` laufen jetzt als root, sie
  bauen keine Verbindungen auf.

## Quellen

CLIProxyAPI @ `8ef43e4df3b216a42493105d31c2873b69191473`: `cmd/server/main.go` (Flag
`-local-model`, `modelCatalogUpdaterPlan`), `internal/misc/antigravity_version.go`,
`internal/util/ssh_helper.go`. Anthropic-IP-Bereiche laut Anthropic-Dokumentation
„IP addresses“ (`160.79.104.0/23`, `2607:6bc0::/48`).

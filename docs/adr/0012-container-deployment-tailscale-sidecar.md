# ADR-0012: Container-Deployment mit Tailscale-Sidecar und `just setup`

- **Status:** Teilweise abgelöst durch [ADR-0013](0013-veroeffentlichung-auf-host-tailnet-ip.md) (Punkte 1 und 6)
- **Datum:** 2026-10-04
- **Issue:** #57

## Kontext

Bisher definiert `docker-compose.yml` nur CLIProxyAPI. tollgate lief als Binary auf dem Host.
[ARCHITECTURE.md](../ARCHITECTURE.md) Abschnitt 5 lässt offen, wie tollgate im Container an
die Tailnet-IP bindet (Host-Netzwerk oder Tailscale-Sidecar). Nach dem ersten echten Test
soll die ganze App per `just setup` eingerichtet werden: Voraussetzungen und Keys prüfen,
dann per Docker deployen, alles im Container. Die harten Regeln gelten unverändert:
tollgate bindet nur an die Tailnet-IP, CLIProxyAPI nur an `localhost`, im auth-dir liegt nur
die Token-Datei des aktiven Accounts.

## Entscheidung

1. **Tailscale-Sidecar.** Ein Dienst `tailscale` (Image auf Version und Digest gepinnt,
   Kernel-Modus mit `/dev/net/tun` und `NET_ADMIN`) tritt dem Tailnet als eigenes Gerät
   `tollgate` bei. `cliproxyapi` und `tollgate` laufen mit `network_mode: service:tailscale`
   in dessen Netzwerk-Namespace:
   - CLIProxyAPI bindet weiterhin `127.0.0.1:8317`. Es ist so nur für tollgate erreichbar,
     nicht für das Tailnet.
   - tollgate bindet die IPv4-Adresse von `tailscale0`. Die Startprüfung aus ADR-0008
     (konkrete Tailnet-IP) läuft ohne Dev-Flag.
   Der Auth-Key kommt per `TS_AUTHKEY=file:/run/secrets/ts_authkey`, der Gerätezustand liegt
   in einem Volume. Ein Neustart braucht den Key deshalb nicht erneut.
2. **tollgate-Image.** Mehrstufiger Build aus dem Repo-Root: Frontend (Node, pnpm über
   Corepack), Binary (`CGO_ENABLED=0`), Laufzeit `debian:bookworm-slim`. Der Entrypoint
   startet als root, wartet auf die Tailnet-IP und kopiert die Secrets mit Modus 0600 auf ein
   tmpfs. Dann schreibt er die Bootstrap-Konfiguration und startet tollgate per `setpriv` als
   Benutzer `tollgate` (UID 10002) mit `no_new_privs`. Grund für den root-Start: Docker
   bindet file-basierte Secrets mit den Rechten der Host-Datei ein. Der unprivilegierte
   Benutzer könnte eine 0600-Datei des Host-Benutzers sonst nicht lesen, und tollgate
   verlangt für Geheimnisdateien 0600 (ADR-0008).
3. **Secrets.** `deploy/setup.sh` legt unter `secrets/` (gitignored, Verzeichnis 0700) die
   fehlenden Werte zufällig an: `tollgate_api_key`, `management_secret`, `admin_token`. Den
   `ts_authkey` fragt es verdeckt ab. Die Dateien selbst sind 0644, weil die Container mit
   eigenen UIDs lesen; das Verzeichnis hält andere Host-Benutzer fern. Kein Secret steht in
   `docker-compose.yml`, keines wird ausgegeben. Einzige Ausnahme ist ein neuer Client-Key:
   `just client` zeigt ihn genau einmal.
4. **Claude-Login im Container.** Ein Dienst `cpa-login` (Profil `login`, dasselbe Image wie
   CLIProxyAPI) führt `cli-proxy-api -claude-login -no-browser` aus und veröffentlicht nur
   `127.0.0.1:54545` für den OAuth-Callback. Das Token landet im Volume des auth-dir.
   `just import-token` übernimmt alternativ eine vorhandene Token-Datei. Beide verweigern
   die Arbeit, wenn schon ein Token im auth-dir liegt (ein aktiver Account; der Wechsel
   folgt mit #28).
5. **`just` als Einstieg.** Das versionierte `justfile` enthält nur Betriebsrezepte (`setup`,
   `login`, `import-token`, `client`, `status`, `up`, `down`, `logs`, `smoke`).
   Entwicklerprüfungen bleiben im Makefile (`make test`). Lokales Agent-Werkzeug wird per
   `import? 'tools/ops.just'` eingebunden und nicht versioniert.
6. **Smoke-Test ohne Tailnet.** `deploy/compose.smoke.yml` ersetzt den Sidecar durch einen
   Platzhalter-Container, der nur den Namespace hält und tollgate auf `127.0.0.1`
   veröffentlicht. tollgate bindet dort `eth0` mit Dev-Flag. `just smoke` baut die Images,
   prüft die Routen und den Benutzerwechsel und entfernt alles wieder. Die Datei wird nur
   für diesen Test verwendet.

## Alternativen

- **Host-Netzwerk (`network_mode: host`):** tollgate bindet die Tailnet-IP des Hosts. Unter
  macOS (Docker Desktop, OrbStack) teilt der Container nicht das Netzwerk des Hosts, und die
  Tailnet-Schnittstelle des Hosts ist im Container nicht sichtbar. Verworfen, weil es nur
  auf Linux-Servern funktioniert.
- **Port auf die Host-Tailnet-IP veröffentlichen (`100.x.y.z:8443:8443`):** tollgate
  müsste im Container eine Wildcard oder die Bridge-IP binden. Die Startprüfung verbietet
  beides, und die Host-IP müsste in die Konfiguration. Verworfen.
- **Tailscale im Userspace-Modus:** keine `tailscale0`-Schnittstelle. Eingehende Verbindungen
  würden auf `localhost` weitergeleitet, tollgate müsste also `localhost` binden. Das
  verletzt die Bindungsregel. Verworfen.
- **Login auf dem Host (`go run … -claude-login`) und Kopie ins Volume:** braucht Go auf
  dem Zielrechner. Bleibt als `just import-token` möglich, ist aber nicht der Standardweg.

## Konsequenzen

- Ein Client erreicht tollgate nur, wenn er im selben Tailnet ist und die Tailnet-ACLs das
  Gerät `tollgate` freigeben. Das Frontend öffnet man unter `http://<tailnet-ip>:8443/`.
- Für den ersten Start braucht der Betreiber einen Tailscale-Auth-Key. Das ist der einzige
  Wert, den `just setup` abfragt.
- `docker-compose.yml` schreibt den tollgate-Dienst nicht mehr #11 zu. #11 behält die
  Abnahme und `docs/CLIENT_SETUP.md`.
- Ein Image-Update von Tailscale ändert Version und Digest an einer Stelle in
  `docker-compose.yml`.

## Quellen

- Tailscale-Container (`tailscale/tailscale:v1.102.5`, Digest `sha256:c507f3a2…`):
  `tailscale up --help`, `--auth-key` mit Präfix `file:`.
- CLIProxyAPI `internal/auth/claude/anthropic_auth.go` (`RedirectURI`, Port 54545),
  `internal/auth/claude/oauth_server.go` (Callback-Server auf allen Schnittstellen,
  Port 54545), Stand siehe [README](README.md#referenzstände).

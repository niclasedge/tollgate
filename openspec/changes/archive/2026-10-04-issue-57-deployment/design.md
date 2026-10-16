# Design

Die tragenden Entscheidungen stehen in [ADR-0012](../../../docs/adr/0012-container-deployment-tailscale-sidecar.md): Tailscale-Sidecar statt Host-Netzwerk, root-Start des Entrypoints nur zum Lesen der Secrets und Finden der Adresse, Login-Dienst mit Callback auf `127.0.0.1:54545`, versioniertes `justfile`.

## Ablauf `just setup`

1. Werkzeuge: `docker`, laufender Daemon, `docker compose` v2, `curl`.
2. Secrets in `secrets/` (0700): `tollgate_api_key`, `management_secret`, `admin_token` zufällig, falls fehlend; `ts_authkey` verdeckt abgefragt oder aus `TS_AUTHKEY`, Format `tskey-…` geprüft.
3. `docker compose --profile login build`.
4. Token-Zählung im auth-Volume über den Login-Dienst (`--no-deps`, ohne Ports). 0 → Login, 1 → weiter. Mehr als eins lässt schon `render-config.sh` scheitern.
5. `docker compose up -d --wait`, dann die Adresse aus `/run/tollgate/listen` lesen und ausgeben.

## Entrypoint

Wartet bis zu 120 s auf IPv4 an `TOLLGATE_LISTEN_IFACE` (Default `tailscale0`), kopiert `tollgate_api_key` und `admin_token` per `install -m 600 -o tollgate`, schreibt `tollgate.yaml` (Listen = IP:8443, Upstream 127.0.0.1:8317, DB `/data/tollgate.db`) und startet `setpriv --reuid=tollgate --regid=tollgate --init-groups --no-new-privs tollgate`. tollgate ist damit PID 1 und erhält SIGTERM direkt.

## Smoke-Test

`deploy/compose.smoke.yml` ersetzt den Sidecar durch `debian:bookworm-slim` mit `sleep infinity` und veröffentlicht `127.0.0.1:18443`. tollgate bindet `eth0` mit Dev-Flag. Geprüft werden Frontend 200, 401 ohne Key und ohne Admin-Token, Client-Anlage, eine durchgereichte Anfrage (nicht 401), Token-Kopie 0600 und Benutzer `tollgate` für PID 1. Eigenes Compose-Projekt und eigenes Secret-Verzeichnis, danach `down -v`.

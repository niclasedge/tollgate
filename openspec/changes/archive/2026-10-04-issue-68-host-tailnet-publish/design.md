# Design

## Bindung

Ohne Sidecar sieht der Container die Tailnet-Schnittstelle des Hosts nicht (Docker Desktop und OrbStack laufen in einer VM). tollgate bindet deshalb die Adresse von `eth0` im Container, und Docker veröffentlicht den Port nur auf der Tailnet-IP des Hosts. Von außen ist tollgate damit nur über das Tailnet erreichbar. Die Regel „bindet nur an die Tailnet-IP“ gilt für den veröffentlichten Socket (ADR-0013).

`config.CheckBinding(listen, published, dev)`:

- `published` leer: unverändert `CheckListen(listen, dev)`.
- `published` gesetzt: `CheckListen(published, false)` (immer streng, auch mit Dev-Flag), dazu für `listen` eine konkrete IP mit Port, keine Wildcard, kein Loopback, und sie MUST privat sein (`10/8`, `172.16/12`, `192.168/16`, `fc00::/7`).

`server.Listen` ruft `CheckBinding` vor dem Öffnen. Das Log `tollgate: listening` nennt zusätzlich `published`.

## Compose

- `cliproxyapi` hält den Namespace und veröffentlicht `${TOLLGATE_HOST_IP}:8443:8443`. CLIProxyAPI selbst bindet nur `127.0.0.1:8317`, über 8443 ist also nur tollgate erreichbar.
- `tollgate` läuft mit `network_mode: service:cliproxyapi` und bekommt `TOLLGATE_PUBLISHED=${TOLLGATE_HOST_IP}:8443`.
- `TOLLGATE_HOST_IP` steht in `.env` (gitignored), damit auch `docker compose down`, `logs` und `just up` die Datei interpolieren können. Fehlt der Wert, bricht Compose mit einem Hinweis auf `just setup` ab.
- Der Smoke-Test ersetzt den Port per `!override` durch `127.0.0.1:18443` und startet tollgate mit Dev-Flag ohne `published`.

## Entrypoint

Liest die IPv4-Adresse von `eth0` (Default von `TOLLGATE_LISTEN_IFACE`), schreibt `published` in die Konfiguration, wenn `TOLLGATE_PUBLISHED` gesetzt ist, und legt die Client-Adresse in `/run/tollgate/published` ab. `/run/tollgate/listen` bleibt die Container-Adresse für den Healthcheck.

## Tailnet-IP des Hosts

`setup.sh` fragt `tailscale ip -4`, unter macOS auch `/Applications/Tailscale.app/Contents/MacOS/Tailscale ip -4`, nimmt die erste Adresse in `100.64.0.0/10` und bricht sonst mit einem verständlichen Hinweis ab.

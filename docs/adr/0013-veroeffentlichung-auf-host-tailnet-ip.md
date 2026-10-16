# ADR-0013: Veröffentlichung auf der Tailnet-IP des Hosts statt Tailscale-Sidecar

- **Status:** Angenommen
- **Datum:** 2026-10-04
- **Issue:** #68
- **Löst ab:** [ADR-0012](0012-container-deployment-tailscale-sidecar.md), Punkt 1 (Tailscale-Sidecar)
  und Punkt 6 (Smoke-Test). Die übrigen Punkte gelten weiter.

## Kontext

Der Stack läuft auf einem Rechner, der selbst im Tailnet ist. Der Sidecar aus ADR-0012 machte
daraus ein zweites Tailnet-Gerät. Er verlangte einen Auth-Key und Kernel-Rechte (`NET_ADMIN`,
`/dev/net/tun`), nur damit tollgate im Container eine Tailnet-IP binden kann. ADR-0012 hatte die
Veröffentlichung auf der Host-Tailnet-IP verworfen, weil die Startprüfung aus ADR-0008 eine
Container-Adresse ablehnt. Das ist eine Frage der Prüfung, nicht der Erreichbarkeit: Von außen
ist tollgate in beiden Varianten nur über das Tailnet erreichbar.

## Entscheidung

1. **Kein Tailscale-Container.** `deploy/setup.sh` ermittelt die IPv4 des Hosts in
   `100.64.0.0/10` (`tailscale ip -4`, unter macOS auch
   `/Applications/Tailscale.app/Contents/MacOS/Tailscale`) und schreibt sie als
   `TOLLGATE_HOST_IP` nach `.env` (gitignored). `just setup` und `just up` aktualisieren den
   Wert. Ohne Tailnet-IP bricht das Setup ab.
2. **Ein Port, nur auf der Tailnet-IP.** `cliproxyapi` hält den gemeinsamen Netzwerk-Namespace
   und veröffentlicht `${TOLLGATE_HOST_IP}:8443:8443`. CLIProxyAPI bindet weiter nur
   `127.0.0.1:8317` und ist über 8443 nicht erreichbar. `tollgate` läuft mit
   `network_mode: service:cliproxyapi`. Fehlt `TOLLGATE_HOST_IP`, bricht Compose mit einem
   Hinweis auf `just setup` ab.
3. **Startprüfung mit `published`.** Die Bootstrap-Konfiguration bekommt `published` (Env
   `TOLLGATE_PUBLISHED`), die Host-Adresse, unter der Docker den Port veröffentlicht.
   `config.CheckBinding`:
   - ohne `published` unverändert `CheckListen` (ADR-0008),
   - mit `published`: `published` MUST in den Tailnet-Bereichen liegen, auch mit Dev-Flag, und
     `listen` MUST eine konkrete private Adresse sein (`10/8`, `172.16/12`, `192.168/16`,
     `fc00::/7`). Wildcard und Loopback werden abgelehnt.
   Die harte Regel „tollgate bindet nur an die Tailnet-IP“ gilt damit für den veröffentlichten
   Socket.
4. **Smoke-Test im Produktionsmodus.** `deploy/compose.smoke.yml` veröffentlicht auf
   `127.0.0.1` und setzt eine Stellvertreter-Adresse `100.64.0.1:8443` als `published`.
   tollgate startet ohne Dev-Flag, die Bindungsprüfung läuft also wie in Produktion.

## Alternativen

- **Sidecar behalten (ADR-0012):** Funktioniert auch auf Rechnern ohne Tailscale, kostet aber
  ein zweites Gerät, einen Auth-Key und Kernel-Rechte. Für den Betrieb auf einem Rechner im
  Tailnet verworfen. Bei Bedarf als eigenes Issue.
- **`listen` mit Dev-Flag statt `published`:** Würde die Bereichsprüfung ganz aufheben. Eine
  Fehlkonfiguration (Port auf `0.0.0.0` veröffentlicht) bliebe unbemerkt. Verworfen.
- **Nur `127.0.0.1` veröffentlichen:** Dann erreichen nur Clients auf demselben Rechner
  tollgate, iPhone und andere Geräte im Tailnet nicht. Verworfen.

## Konsequenzen

- `just setup` fragt nichts mehr ab, wenn Secrets und Token vorhanden sind. Für einen
  Neustart reicht Tailscale auf dem Host.
- tollgate prüft nicht selbst, worauf Docker den Port tatsächlich veröffentlicht.
  `published` ist eine Zusicherung von Compose. Compose übernimmt `TOLLGATE_HOST_IP` für den
  Port und für `TOLLGATE_PUBLISHED` aus derselben Variablen.
- Prozesse auf dem Host selbst erreichen den Container auch über seine Bridge-Adresse. Andere
  Geräte erreichen ihn nur über die Tailnet-IP. Client-Key und Admin-Token gelten unverändert.
- Ändert sich die Tailnet-IP des Hosts oder läuft Tailscale beim Start nicht, startet der
  Port nicht. `just up` schreibt die IP neu.

# tollgate deployen

Alles läuft in Containern (tollgate und CLIProxyAPI). Docker veröffentlicht tollgate nur auf
der Tailnet-IP dieses Rechners. Die Entscheidungen stehen in
[ADR-0012](adr/0012-container-deployment-tailscale-sidecar.md) und
[ADR-0013](adr/0013-veroeffentlichung-auf-host-tailnet-ip.md). Einstieg ist `just setup`.

## Voraussetzungen

- Docker mit `docker compose` v2 (Docker Desktop, OrbStack oder Docker Engine), `just`, `curl`.
- Tailscale läuft auf diesem Rechner (`tailscale ip -4` oder die Tailscale-App unter macOS).
- Ein Claude-Abo für den Login.

## Einrichten

```sh
just setup
```

1. Prüft Docker, Compose, curl und die Tailnet-IP dieses Rechners und schreibt sie nach `.env`.
2. Legt fehlende Secrets unter `secrets/` an.
3. Baut die Images.
4. Liegt noch kein Claude-Token im auth-dir, startet es den Login. Öffne die angezeigte URL im
   Browser des Rechners, auf dem `just setup` läuft (Callback auf `127.0.0.1:54545`).
5. Startet den Stack, wartet auf Health und zeigt die Adresse von tollgate.

Ein vorhandenes CLIProxyAPI-Token statt des Logins: `just import-token <datei.json>` vor
`just setup`.

## Client anlegen

```sh
just client macbook
```

Das Ergebnis sind die beiden Variablen für Claude Code (der Key erscheint nur dieses eine Mal):

```sh
export ANTHROPIC_BASE_URL=http://<tailnet-ip>:8443
export ANTHROPIC_AUTH_TOKEN=tg_...
claude
```

## Betrieb

| Befehl | Wirkung |
|---|---|
| `just status` | Container, Tailnet-Adresse, Token vorhanden? |
| `just logs` | Logs aller Dienste |
| `just down` / `just up` | Stoppen / Starten; Token und Datenbank bleiben in Volumes, `up` aktualisiert die Tailnet-IP |
| `just smoke` | Stack ohne Tailnet bauen und prüfen, danach entfernen |
| `just slack-secret` | Slack-Token und Kanal für den Limit-Alarm schreiben |

Das Frontend liegt unter `http://<tailnet-ip>:8443/`, der Admin-Token in `secrets/admin_token`.

Hinweise:

- tollgate ist nur über die Tailnet-IP erreichbar, auch vom Rechner selbst: nicht über
  `localhost` und nicht über die LAN-Adresse.
- `just login` funktioniert auch ohne Terminal (z. B. per ssh), die Login-URL steht dann im
  Log.
- tollgate hängt am Netzwerk-Namespace von CLIProxyAPI. Wird der CLIProxyAPI-Container neu
  erzeugt (z. B. nach einem Image-Update), danach `just down` und `just up`.
- Ändert sich die Tailnet-IP dieses Rechners, schreibt `just up` sie neu.
- Redaktion ein- oder ausschalten: in `.env` die Zeile `TOLLGATE_REDACT_ENABLED=true` (oder
  `false`) setzen, danach `just up`. Ohne Zeile bleibt sie aus.
- Kompression von Bash-Ausgaben ist im Container an. Ausschalten: in `.env` die Zeile
  `TOLLGATE_COMPRESS_ENABLED=false` setzen, danach `just up`. Beim Ein- oder Ausschalten verfehlen laufende Sitzungen den Prompt-Cache einmal, weil sich
  ihr Verlauf ändert.
- Nach einem Neustart des Rechners kann Docker vor Tailscale starten. Dann fehlt die
  Tailnet-IP, der Port lässt sich nicht binden und tollgate bleibt aus. Abhilfe: `just up`,
  sobald Tailscale verbunden ist. `just status` zeigt den Zustand auch ohne Tailscale.

## Ausgehende Verbindungen

Der Netz-Namespace von CLIProxyAPI und tollgate erreicht nur Anthropic
([ADR-0017](adr/0017-cpa-egress-nur-anthropic.md)). Der Entrypoint von CLIProxyAPI setzt
beim Start mit `iptables`/`ip6tables` `OUTPUT DROP` und erlaubt nur `tcp/443` zu
`160.79.104.0/23` und `2607:6bc0::/48`, Loopback, Antworten und DNS über Docker. Danach läuft
CLIProxyAPI als Benutzer `cpa` ohne Capabilities und mit `-local-model` (kein entfernter
Modellkatalog). Dafür bekommen `cliproxyapi` und `cpa-login` `NET_ADMIN`.

- Lassen sich die Regeln nicht setzen (fehlendes `NET_ADMIN`, Kernel ohne
  `nf_tables`/`ip_tables` für IPv4 und IPv6), startet der Container nicht. Im Log steht
  `cpa-entrypoint: …, not starting`.
- Im Log von CLIProxyAPI stehen gelegentlich Timeouts zu GitHub oder anderen Diensten
  (Versions-Updater, IP-Lookup beim Login). Das ist gewollt.
- Prüfen: `docker compose exec tollgate timeout 8 bash -c 'exec 3<>/dev/tcp/example.com/443'`
  muss scheitern, mit `api.anthropic.com` gelingen. `just smoke` prüft beides.
- Optional auf Linux-Hosts als zweite Schicht: Regeln in der Kette `DOCKER-USER`, die nur
  `tcp/443` zu den Anthropic-Bereichen aus dem Compose-Netz durchlassen. Unter Docker
  Desktop und OrbStack gibt es diese Kette auf dem Host nicht.

## Limit-Alarm per Slack

tollgate warnt ab 90 % im 5-Stunden-Fenster des aktiven Abos, je höherem Prozentpunkt mit
einer Nachricht. Sie nennt ein anderes Abo mit freiem Wochenlimit. Lehnt Anthropic Anfragen
ab, wechselt tollgate automatisch auf dieses Abo und meldet das
([ADR-0019](adr/0019-alarme-spool-notifier.md), #122). Der Wechsel braucht keinen Slack.

Die Nachrichten schickt der Dienst `notifier`. Er liest Bot-Token und Kanal-ID aus
`secrets/slack_bot_token` und `secrets/slack_channel`. Sind sie leer, ist Slack aus.

Liegen `SLACK_BOT_TOKEN` und `SLACK_ALERT_CHANNEL` in einer SOPS-verschlüsselten
dotenv-Datei, reicht eine Zeile in `.env`. Dann füllen `just setup` und
`just slack-secret` die Secrets selbst, ohne die Werte auszugeben:

```sh
TOLLGATE_SLACK_SOPS_FILE=~/pfad/zu/secrets.env
```

Der age-Key kommt aus `SOPS_AGE_KEY_FILE`, ohne Angabe aus
`~/.config/sops/age/keys.txt` (sops sucht auf macOS sonst nur unter
`~/Library/Application Support/sops/age/`). Ohne SOPS geht es direkt:
`SLACK_BOT_TOKEN=… SLACK_ALERT_CHANNEL=… just slack-secret`.

Der Bot braucht den Scope `chat:write` und muss im Kanal Mitglied sein. Der Notifier liest
die Dateien bei jedem Durchlauf (alle 10 s), ein Neustart ist nicht nötig. Fehler wie
`slack: channel_not_found` stehen in `just logs`.

Einstellungen in `.env`, danach `just up`:

| Variable | Default | Wirkung |
|---|---|---|
| `TOLLGATE_ALERTS_AUTO_SWITCH` | `false` | automatischer Wechsel bei Ablehnung; ohne ihn bietet die Übersicht den Wechsel als Knopf an |
| `TOLLGATE_ALERTS_FIVE_HOUR_THRESHOLD` | `90` | erste Warnung ab diesem Prozentwert |

## Basis-Images aktualisieren

Jede `FROM`-Zeile in `deploy/tollgate/Dockerfile` und `deploy/cliproxyapi/Dockerfile` nennt
Tag und Digest des Multi-Arch-Index (`image:tag@sha256:…`). Docker zieht nur den Digest; der
Tag zeigt, woher er stammt. Ein Rebuild holt damit nie unbemerkt ein anderes Basis-Image.
`deploy/baseimages_test.go` (Teil von `make test`) lässt jede `FROM`-Zeile ohne Digest
scheitern. Aktualisiert wird von Hand, als eigenes Issue:

1. Aktuellen Index-Digest je Tag lesen (Feld `Digest` oben, nicht die Plattform-Manifeste):
   ```sh
   for i in node:22-bookworm-slim golang:1.26-bookworm debian:bookworm-slim; do
     printf '%s %s\n' "$i" "$(docker buildx imagetools inspect "$i" --format '{{.Manifest.Digest}}')"
   done
   ```
2. Den Digest hinter dem Tag ersetzen. `golang` und `debian` stehen in beiden Dockerfiles und
   bekommen denselben Digest. Ein neuer Tag (z. B. Go-Version) ist ein eigener Schritt; bei Go
   dazu `go.mod` und CI prüfen.
3. `make test`, `docker compose build` und `just smoke`.

## Accounts hinzufügen und wechseln

Unter **Accounts → Account hinzufügen** wird eine Login-URL angezeigt. Den Link öffnen,
beim gewünschten Claude-Account anmelden und freigeben. Danach die vollständige
`http://localhost:54545/callback?…`-URL aus der Browser-Adresszeile zurück in das
Callback-Feld kopieren und **Account speichern** wählen. Der localhost-Callback muss
nicht erreichbar sein; das funktioniert auch auf dem iPhone. Der Link gilt zehn Minuten.
Ein bereits angemeldeter Browser kann den bisherigen Account verwenden: für das zweite
Abo ein privates Fenster verwenden oder bei Claude vorher abmelden.

Alternativ lässt sich eine bestehende CLIProxyAPI-Claude-Token-Datei importieren. Beide
Wege speichern im verschlüsselten Tresor und ändern den aktiven Account zunächst nicht.
Ein bereits vorhandener aktiver Account wird automatisch übernommen.

Beim gewünschten Account **Aktivieren → Wechseln und testen** wählen. Falls noch eine
Anfrage/Streaming-Antwort läuft, wird der Wechsel vorgemerkt und nach deren Ende
automatisch ausgeführt. Neue Proxy-Anfragen warten während Wartephase und Wechsel
und verwenden anschließend den neuen Account. Die Oberfläche zeigt die Anzahl
noch laufender Anfragen. Ein laufender Account-Test wird ebenfalls abgewartet;
eine zweite gleichzeitig wartende Vormerkung überschreibt die erste nicht.
Nach der Aktivierung läuft automatisch ein kurzer Sonnet-5.5-Beispielaufruf. Der Hinweis zeigt
Wechsel und Testergebnis getrennt: ein Rate-Limit im Test macht den Wechsel nicht rückgängig.
Der aktive Account hat außerdem **Beispielaufruf testen**. Die Tests verbrauchen wenige
Tokens. Inaktive Accounts können nach Inline-Bestätigung entfernt werden.

Bei einem Fehler beim Tausch versucht tollgate den alten Account wiederherzustellen und
zeigt das Ergebnis an. Die Refresh-Barriere wartet eine laufende Token-Erneuerung ab; erst dann werden die zuletzt aktualisierten Tokens vor dem Entfernen gesichert.
Bei einer Unterbrechung zwischen Entfernen und Hochladen bleiben beide Accounts im Tresor;
die gewünschte Datei kann durch erneutes Aktivieren wiederhergestellt werden.

**Backup:** Im Volume `tollgate-data` liegen `/data/tollgate.db` und `/data/crypto.key`.
Beide gemeinsam sichern; die Key-Datei ist 32 Byte groß und hat Rechte 0600. Ohne diesen
Schlüssel sind die verschlüsselten Accounts nicht lesbar. Bei bestehendem Tresor erzeugt
tollgate keinen Ersatzschlüssel. Im Volume `cliproxyapi-auth` bleibt genau die aktive
Token-Datei. Diese Volumes nicht löschen, um einen Account zu wechseln.

Details und Protokollquelle: [ADR-0015](adr/0015-account-tresor-und-wechsel.md).

Wenn der Beispielaufruf meldet, dass Anthropic OAuth für die Organisation nicht erlaubt
(HTTP 403), war der Login erfolgreich, aber API-Nutzung mit diesem Abo ist aktuell
blockiert. Der Account bleibt im Tresor. Über **Aktivieren** lässt sich wieder zu einem
anderen Account wechseln. tollgate ändert diese Organisationsberechtigung nicht.

### Wochenlimits je Account

Jede Account-Karte liest die 5-Stunden- und Wochenlimits separat über den festen
OAuth-Usage-Endpunkt. Das aktiviert keinen anderen Account und erzeugt keinen
Modellaufruf. Der Backend-Cache hält erfolgreiche Messwerte fünf Minuten; die
Accounts-Ansicht prüft alle 30 Sekunden auf neue Werte. Letzte erfolgreiche
Messwerte bleiben nach einem Neustart erhalten. Die API speichert ausschließlich
validierte Prozente und Reset-Zeiten, keine vollständigen Upstream-Antworten.

Die Jetzt-Markierung zeigt die vergangene Zeit in der vom Reset rückwärts
berechneten 7-Tage-Periode; sie ist kein geschätzter Verbrauch. Der gefüllte Balken
zeigt den tatsächlich gemeldeten Verbrauch. Alte Daten und überschrittene Resets
werden gekennzeichnet. Bei abgelaufenem Access-Token eines inaktiven Accounts wird
kein automatischer Refresh ausgelöst; die Ansicht zeigt den letzten Messstand und
fordert bei HTTP 401 eine erneute Anmeldung an. Der aktive Account verwendet das
aktuelle Access-Token aus CPA, damit laufende Token-Rotation berücksichtigt wird.

### Account-Zuordnung der Anfragen

Neue Traces speichern `account_id` als unveränderliche Momentaufnahme des aktiven
Accounts. Die ID wird nach Eintritt in die Wechsel-Sperre gelesen; laufende Streams
behalten ihre Zuordnung bis zum Abschluss. Client-Header bestimmen diese ID nicht.
Live zeigt eine Kurz-ID, die aufgeklappte Anfrage die vollständige ID. Die
Kostenansicht und `/api/costs` trennen Verbrauch und USD-Listenpreisvergleich nach
Account. Alte Anfragen bleiben unzugeordnet; ein gelöschter Account löscht keine
historischen Zuordnungen. Ein Identitäts-Read, der fehlschlägt, stoppt die Anfrage
vor der Weiterleitung mit HTTP 503 statt einer geratenen Zuordnung.

Die Account-Karten verwenden zusätzlich die Wochenlimits aus echten Antworten
und Aktivierungs-/Beispielaufrufen. Diese werden anhand derselben Account-ID
zugeordnet und im Hintergrund gespeichert. Ein gedrosselter Usage-Endpunkt
überschreibt diesen Messstand nicht. Teilantworten behalten alte Fenster samt
ursprünglichem Messzeitpunkt; ein frischer 5-Stunden-Wert macht keine alte Woche
frisch. Dadurch kann die Account-Karte auch bei Usage-HTTP-429 aktuelle Limits
anzeigen, sobald eine echte Antwort ihre Limits liefert.

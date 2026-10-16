# deployment Specification

## Purpose
Einrichtung und Betrieb von tollgate in Containern: `just setup` prüft Voraussetzungen, Secrets und die Tailnet-IP des Hosts, meldet das Claude-Abo an und startet tollgate und CLIProxyAPI; Docker veröffentlicht tollgate nur auf der Tailnet-IP des Hosts (ADR-0012, ADR-0013).

## Requirements

### Requirement: Einrichtung mit einem Befehl

`just setup` SHALL Docker, den laufenden Daemon, `docker compose` v2, `curl` und eine Tailnet-IPv4 des Hosts (`tailscale ip -4`, unter macOS auch die Tailscale-App) prüfen, die Tailnet-IP nach `.env` schreiben, fehlende Secrets unter `secrets/` anlegen, ohne vorhandene zu überschreiben, bei fehlendem Claude-Token den Login starten und den Stack erst danach mit Warten auf Health starten. Es MUST NOT einen Tailscale-Auth-Key verlangen. Secrets MUST NOT ausgegeben werden.

#### Scenario: Erster Start
- **WHEN** `secrets/` fehlt, der Host im Tailnet ist und kein Token im auth-dir liegt
- **THEN** legt `just setup` die Secrets an, startet den Login und danach den Stack und gibt die Tailnet-Adresse von tollgate aus.

#### Scenario: Wiederholter Start
- **WHEN** Secrets und Token vorhanden sind
- **THEN** überschreibt `just setup` keine Secrets, überspringt den Login und startet den Stack.

#### Scenario: Host nicht im Tailnet
- **WHEN** keine Tailnet-IPv4 des Hosts ermittelt werden kann
- **THEN** bricht `just setup` vor dem Start mit einem Hinweis ab, Tailscale auf dem Host zu starten.

### Requirement: Alles im Container mit eingehaltenen Bindungsregeln

Der Stack SHALL tollgate und CLIProxyAPI in einem gemeinsamen Netzwerk-Namespace enthalten, ohne Tailscale-Container. Docker MUST tollgate nur auf der Tailnet-IP des Hosts veröffentlichen (`<tailnet-ip>:8443`), und tollgate MUST mit `published` auf diese Adresse starten. CLIProxyAPI MUST nur `127.0.0.1` binden. Außer diesem Port und dem OAuth-Callback des Login-Dienstes auf `127.0.0.1:54545` MUST kein Port veröffentlicht werden. tollgate MUST ohne Root laufen.

#### Scenario: Smoke-Test ohne Tailnet
- **WHEN** `just smoke` läuft
- **THEN** antwortet tollgate auf `/` mit 200, ohne Key mit 401, die Admin-API ohne Token mit 401, ein angelegter Client erreicht CLIProxyAPI, die Token-Kopie hat Modus 0600 und PID 1 läuft als Benutzer `tollgate`; danach ist der Stack entfernt.

### Requirement: Ein aktiver Account

`just login` und `just import-token` MUST abbrechen, wenn im auth-dir bereits eine Token-Datei liegt.

#### Scenario: Zweites Token
- **WHEN** ein Token vorhanden ist und `just import-token` aufgerufen wird
- **THEN** bricht der Befehl mit einem Hinweis auf den einen aktiven Account ab, ohne das auth-dir zu ändern.

### Requirement: Client-Snippet

`just client <name>` SHALL über die Admin-API einen Client anlegen und `ANTHROPIC_BASE_URL` und `ANTHROPIC_AUTH_TOKEN` ausgeben. Der Name MUST auf Buchstaben, Ziffern, `.`, `_` und `-` beschränkt sein.

#### Scenario: Client anlegen
- **WHEN** der Stack läuft und `just client macbook` aufgerufen wird
- **THEN** erscheinen beide Variablen mit der Tailnet-Adresse und dem neuen Key.

### Requirement: Basis-Images per Digest gepinnt

Jede `FROM`-Zeile eines Dockerfiles im Repository MUST ein externes Image mit Tag und Digest als `image:tag@sha256:<64 Hex-Zeichen>` nennen oder auf eine zuvor im selben Dockerfile mit `AS` benannte Stage verweisen. Der Digest SHALL der Digest des Multi-Arch-Index sein. `docs/DEPLOY.md` MUST beschreiben, wie die Digests ermittelt und aktualisiert werden.

#### Scenario: Floating Tag wird abgelehnt
- **WHEN** ein Dockerfile `FROM debian:bookworm-slim` ohne Digest enthält
- **THEN** schlägt `make test` mit Datei, Zeile und Image fehl.

#### Scenario: Gepinnte Images bestehen
- **WHEN** alle `FROM`-Zeilen in `deploy/tollgate/Dockerfile` und `deploy/cliproxyapi/Dockerfile` Tag und Digest tragen
- **THEN** besteht der Test, und beide Dateien wurden geprüft.

#### Scenario: Stage-Verweis bleibt erlaubt
- **WHEN** eine `FROM`-Zeile eine früher mit `AS` benannte Stage nennt
- **THEN** verlangt der Test dafür keinen Digest.

#### Scenario: Update nachvollziehbar
- **WHEN** ein Reviewer `docs/DEPLOY.md` liest
- **THEN** findet er den Befehl zum Ermitteln des Index-Digests und die Schritte bis zum Smoke-Test.

### Requirement: Egress des CPA-Namespaces nur zu Anthropic

Der Entrypoint des CLIProxyAPI-Images SHALL vor dem Start von CLIProxyAPI für IPv4 und IPv6 in der Kette `OUTPUT` der Tabelle `filter` zuerst die Policy `DROP` setzen und danach nur Ausgang über `lo`, Pakete im Zustand `ESTABLISHED,RELATED`, `tcp/443` zu `160.79.104.0/23` bzw. `2607:6bc0::/48`, DNS (`udp`/`tcp` 53) nur für Sockets von uid 0 und bei IPv6 ICMPv6-Nachbarerkennung erlauben. Die Tabelle `nat` MUST unberührt bleiben. Schlägt eine Regel fehl oder ist weder `iptables` noch `iptables-legacy` nutzbar, MUST der Entrypoint mit einem Fehler enden, ohne die Konfiguration zu rendern oder CLIProxyAPI zu starten. Danach MUST er mit `setpriv` auf den Benutzer `cpa` ohne Bounding- und Inheritable-Capabilities und mit `no_new_privs` wechseln. Compose MUST nur `cliproxyapi` und `cpa-login` die Capability `NET_ADMIN` geben; tollgate MUST keine zusätzlichen Capabilities erhalten und erbt die Regeln über den gemeinsamen Namespace.

#### Scenario: Regeln vor dem Start
- **WHEN** der Entrypoint mit nutzbarem `iptables` läuft
- **THEN** ist der erste Regelaufruf je Familie `-P OUTPUT DROP`, es folgen genau die erlaubten Regeln, und erst danach startet CLIProxyAPI als `cpa` ohne Capabilities.

#### Scenario: Fail-closed
- **WHEN** eine `iptables`- oder `ip6tables`-Regel fehlschlägt oder kein Backend nutzbar ist
- **THEN** endet der Entrypoint mit Exit-Code ungleich 0, und weder `render-config.sh` noch CLIProxyAPI werden aufgerufen.

#### Scenario: Legacy-Backend
- **WHEN** `iptables` (nft) `-S OUTPUT` nicht ausführen kann, `iptables-legacy` aber schon
- **THEN** setzt der Entrypoint alle IPv4-Regeln mit `iptables-legacy`.

#### Scenario: Egress im laufenden Stack
- **WHEN** `just smoke` läuft
- **THEN** baut der Namespace eine Verbindung zu `api.anthropic.com:443` auf, eine Verbindung zu einem fremden Host scheitert, und das Image ohne `NET_ADMIN` startet nicht.

### Requirement: Kein entfernter Modellkatalog

Der Entrypoint MUST CLIProxyAPI immer mit `-local-model` starten, auch beim Login. Der Login-Dienst `cpa-login` MUST denselben Entrypoint nutzen und nur `-claude-login -no-browser` als Argumente übergeben.

#### Scenario: Server-Start
- **WHEN** der Entrypoint ohne Argumente läuft
- **THEN** startet CLIProxyAPI mit `-config /run/cpa/config.yaml -local-model`.

#### Scenario: Login
- **WHEN** `cpa-login` startet
- **THEN** startet CLIProxyAPI mit `-local-model -claude-login -no-browser` hinter denselben Egress-Regeln.

### Requirement: Image-Build mit vollständigen Eingaben

`deploy/tollgate/Dockerfile` MUST jede Go- und Embed-Datei der Pakete dieses Moduls, von denen `./cmd/tollgate` abhängt, sowie `go.mod` und `go.sum` per `COPY` an ihren Repo-Pfad unter dem `WORKDIR` der Build-Stage legen, oder sie aus einer früheren Stage per `COPY --from` erhalten. `.dockerignore` MUST keine dieser Dateien ausschließen und SHALL von `testdata` nur `testdata/fixtures` freigeben.

#### Scenario: Fehlende Build-Eingabe wird erkannt
- **WHEN** `cmd/tollgate` ein Paket oder eine Embed-Datei nutzt, die keine `COPY`-Zeile der Build-Stage abdeckt oder die `.dockerignore` ausschließt
- **THEN** schlägt `make test` mit dem Pfad der fehlenden Datei fehl.

#### Scenario: Image baut aus dem Repo-Root
- **WHEN** `docker build -f deploy/tollgate/Dockerfile .` im Repo-Root läuft
- **THEN** läuft `go build` in der Build-Stage durch, und das Image entsteht.

#### Scenario: Nur Fixtures aus testdata
- **WHEN** der Build-Kontext aus `.dockerignore` entsteht
- **THEN** enthält er aus `testdata` nur `testdata/fixtures`.

### Requirement: Redact-Schalter im Container
Der Dienst `tollgate` in `docker-compose.yml` SHALL die Variable `TOLLGATE_REDACT_ENABLED` aus `.env` an den Container durchreichen, mit dem Default `false`, solange #90 den Default nicht ändert.

#### Scenario: Redaktion per .env eingeschaltet
- **WHEN** in `.env` `TOLLGATE_REDACT_ENABLED=true` steht und `just up` läuft
- **THEN** startet tollgate mit eingeschalteter Redact-Stufe

#### Scenario: Ohne Eintrag
- **WHEN** `.env` die Variable nicht setzt
- **THEN** erhält der Container `TOLLGATE_REDACT_ENABLED=false` und die Redaktion bleibt aus

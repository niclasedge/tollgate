## MODIFIED Requirements

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

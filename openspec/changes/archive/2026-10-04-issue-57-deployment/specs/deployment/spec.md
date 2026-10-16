## ADDED Requirements

### Requirement: Einrichtung mit einem Befehl

`just setup` SHALL Docker, den laufenden Daemon, `docker compose` v2 und `curl` prüfen, fehlende Secrets unter `secrets/` anlegen, ohne vorhandene zu überschreiben, den Tailscale-Auth-Key verdeckt abfragen, bei fehlendem Claude-Token den Login starten und den Stack erst danach mit Warten auf Health starten. Secrets MUST NOT ausgegeben werden.

#### Scenario: Erster Start
- **WHEN** `secrets/` fehlt und kein Token im auth-dir liegt
- **THEN** legt `just setup` die Secrets an, fragt den Auth-Key ab, startet den Login und danach den Stack und gibt die Adresse von tollgate aus.

#### Scenario: Wiederholter Start
- **WHEN** Secrets und Token vorhanden sind
- **THEN** überschreibt `just setup` nichts, überspringt den Login und startet den Stack.

### Requirement: Alles im Container mit eingehaltenen Bindungsregeln

Der Stack SHALL tollgate, CLIProxyAPI und einen Tailscale-Sidecar enthalten. tollgate MUST nur die Tailnet-IP des Sidecars binden, CLIProxyAPI MUST nur `127.0.0.1` binden, und außer dem OAuth-Callback des Login-Dienstes auf `127.0.0.1:54545` MUST kein Port veröffentlicht werden. tollgate MUST ohne Root laufen.

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

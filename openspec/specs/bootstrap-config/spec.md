# bootstrap-config Specification

## Purpose
Die Bootstrap-Konfiguration liefert die Startwerte (Bindung, Upstream, Datenbank, Logging), die tollgate vor dem Öffnen der Datenbank braucht, und erzwingt die Bindungsregel nur Tailnet.

## Requirements

### Requirement: Bootstrap-Konfiguration

`config.Load` SHALL die Datei `tollgate.yaml` (Ebene 1 aus ADR-0008) lesen, Defaults setzen (`upstream.url=http://127.0.0.1:8317`, `log.level=info`) und danach die Umgebungsvariablen `TOLLGATE_LISTEN`, `TOLLGATE_PUBLISHED`, `TOLLGATE_UPSTREAM_URL`, `TOLLGATE_UPSTREAM_API_KEY_FILE`, `TOLLGATE_DB_PATH` und `TOLLGATE_LOG_LEVEL` anwenden. Abschnitte, die von späteren Issues stammen, SHALL ignoriert werden. `Validate` MUST `listen`, `upstream.api_key_file` und `db.path` verlangen, `upstream.url` auf Loopback prüfen und ein gültiges `log.level` verlangen.

#### Scenario: Datei und Umgebung
- **WHEN** eine Datei und Umgebungsvariablen denselben Wert setzen
- **THEN** gewinnt die Umgebungsvariable, und fehlende Werte erhalten ihren Default.

#### Scenario: Pflichtwert fehlt
- **WHEN** `listen`, `api_key_file` oder `db.path` fehlt oder die Upstream-URL nicht auf Loopback zeigt
- **THEN** meldet `Validate` einen Fehler, der den Schlüssel nennt.

### Requirement: Startprüfung der Bindung

`config.CheckListen` SHALL nur konkrete IP-Adressen mit Port akzeptieren, die in `100.64.0.0/10` oder `fd7a:115c:a1e0::/48` liegen. Wildcard-Adressen (`0.0.0.0`, `::`, leerer Host, IPv4-mapped Wildcard) und Hostnamen MUST immer abgelehnt werden. Das Flag `--dev-allow-any-listen` SHALL nur die Bereichsprüfung aufheben, nicht die Wildcard-Prüfung, und MUST eine Warnung ins Log schreiben.

`config.CheckBinding(listen, published, dev)` SHALL ohne `published` genau `CheckListen(listen, dev)` sein. Ist `published` gesetzt (die Host-Adresse, unter der Docker den Port veröffentlicht), MUST `published` die Prüfung von `CheckListen` ohne Dev-Flag bestehen, und `listen` MUST eine konkrete private Adresse mit Port sein (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fc00::/7`); Wildcard, Loopback und Hostnamen MUST abgelehnt werden. `server.Listen` MUST `CheckBinding` vor dem Öffnen des Listeners ausführen. Bei Verstoß MUST der Start mit einer JSON-Fehlerzeile und Exit-Code ungleich 0 scheitern.

#### Scenario: Adressen
- **WHEN** `CheckListen` mit `100.64.0.1:8443`, `100.128.0.1:8443`, `127.0.0.1:8443`, `0.0.0.0:8443`, `:8443` und `localhost:8443` aufgerufen wird
- **THEN** wird nur die erste ohne Dev-Flag akzeptiert; mit Dev-Flag wird zusätzlich `127.0.0.1:8443` akzeptiert, Wildcard und Hostname nie.

#### Scenario: Veröffentlichte Adresse
- **WHEN** `CheckBinding` mit `listen: 172.18.0.2:8443` und `published: 100.64.0.1:8443` aufgerufen wird
- **THEN** wird die Bindung akzeptiert; mit `published: 192.168.1.5:8443` (auch mit Dev-Flag) oder `listen: 127.0.0.1:8443` oder `listen: 0.0.0.0:8443` wird sie abgelehnt.

#### Scenario: Start verweigert
- **WHEN** das Binary mit `listen: 0.0.0.0:8443` (auch mit Dev-Flag) gestartet wird
- **THEN** endet es mit Exit ungleich 0, schreibt nichts auf stdout und eine JSON-Fehlerzeile mit `level=ERROR` auf stderr.

### Requirement: Geheimnisdateien

`config.ReadSecretFile` SHALL Schlüsseldateien nur akzeptieren, wenn Gruppe und Andere keinen Zugriff haben (Modus 0600) und der getrimmte Inhalt nicht leer ist. Fehlermeldungen MUST NOT den Inhalt enthalten.

#### Scenario: Zu offene Rechte
- **WHEN** die Datei Modus 0644 hat
- **THEN** meldet `ReadSecretFile` einen Fehler, der 0600 nennt, aber nicht den Schlüssel.

### Requirement: Start, Betrieb und Beenden

`cmd/tollgate` SHALL Konfiguration laden und prüfen, die Schlüsseldatei lesen, den Store öffnen, einen Runner mit Stufe 0 bauen, den Listener nach der Bindungsprüfung öffnen und den Proxy bedienen. Es MUST `tollgate: listening` mit der tatsächlichen Adresse als JSON-Zeile loggen und bei SIGINT/SIGTERM geordnet mit Exit 0 enden. Ein nicht erreichbares CLIProxyAPI SHALL nur eine Warnung auslösen. Das Binary MUST ohne cgo bauen und MUST NOT Pakete unter `third_party/` importieren.

#### Scenario: Echter Start
- **WHEN** das aus einem frischen Checkout gebaute Binary mit Loopback-Adresse und Dev-Flag startet
- **THEN** meldet es die Adresse, beantwortet eine Anfrage ohne Key mit 401 und endet nach SIGTERM mit Exit 0.

### Requirement: Admin-Token-Datei

Die Bootstrap-Konfiguration SHALL `admin.token_file` (Env `TOLLGATE_ADMIN_TOKEN_FILE`) lesen. Fehlt der Wert, MUST der Default `admin.token` im Verzeichnis von `db.path` gelten. Fehlt die Datei beim Start, MUST tollgate sie mit einem zufälligen Token und Modus 0600 anlegen; der Token MUST NOT geloggt werden.

#### Scenario: Erster Start ohne Token-Datei
- **WHEN** tollgate ohne `admin.token_file` und ohne vorhandene Datei startet
- **THEN** liegt danach `admin.token` mit Modus 0600 neben der Datenbank, und die Logzeile nennt nur den Pfad.

#### Scenario: Env überschreibt
- **WHEN** `TOLLGATE_ADMIN_TOKEN_FILE=/a.token` gesetzt ist
- **THEN** verwendet tollgate `/a.token`.

### Requirement: Schalter für die Redact-Stufe

Die Bootstrap-Konfiguration SHALL den Schalter `redact.enabled` (Umgebung `TOLLGATE_REDACT_ENABLED`) lesen. Der Standard MUST `false` sein, solange der Rückweg (#14) fehlt. Ein Umgebungswert, der kein Wahrheitswert ist, MUST mit einer Fehlermeldung abgelehnt werden, die den Variablennamen nennt. Ist der Schalter an, SHALL tollgate die Stufe `redact` nach dem Eingang einhängen und beim Start warnen.

#### Scenario: Standard
- **WHEN** weder Datei noch Umgebung den Schalter setzen
- **THEN** ist `redact.enabled` aus.

#### Scenario: Umgebung gewinnt
- **WHEN** die Datei `redact.enabled: false` und die Umgebung `TOLLGATE_REDACT_ENABLED=true` setzt
- **THEN** ist der Schalter an.

#### Scenario: Ungültiger Wert
- **WHEN** `TOLLGATE_REDACT_ENABLED=vielleicht` gesetzt ist
- **THEN** schlägt `Load` mit einer Meldung fehl, die `TOLLGATE_REDACT_ENABLED` nennt.

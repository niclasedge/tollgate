## MODIFIED Requirements

### Requirement: Weiterleiten der Pipeline-Ausgabe an CLIProxyAPI

Der Proxy SHALL jede Anfrage durch den `pipeline.Runner` führen und danach den von der Pipeline gelieferten Body (nicht den Original-Body) mit derselben Methode, demselben Pfad und derselben Query an die Upstream-URL weiterleiten. Das gilt für alle Pfade, auch für `count_tokens`, Modelllisten und unbekannte Endpunkte. Antwortstatus, Antwortkörper und Antwortheader (ohne Hop-by-hop) MUST unverändert an den Client gehen; Ausnahmen sind ein Upstream-401, den der Proxy MUST als 502 `api_error` ohne Upstream-Körper und -Header beantworten (der Client ist zu diesem Zeitpunkt bereits authentifiziert), und ein Upstream-403 auf `/v1/messages` vom Typ `permission_error` mit Fehlercode `oauth_*`, den der Proxy MUST als 503 `api_error` mit `Retry-After: 15` und `x-should-retry: true` beantworten (das aktive Abo ist gesperrt, ein Abo-Wechsel behebt das; der Client soll wiederholen statt abzubrechen). Bricht ein Antwortstrom nach gesendeten Headern ab, MUST der Proxy die Client-Verbindung abbrechen statt die Antwort regulär zu beenden. Der Proxy MUST Weiterleitungen des Upstream nicht folgen.

#### Scenario: Pipeline-Ausgabe wird weitergeleitet
- **WHEN** eine Stufe einen Text im Body ersetzt und der Client `POST /v1/messages?beta=true` sendet
- **THEN** erhält der Upstream `POST /v1/messages?beta=true` mit dem ersetzten Body, und der Client erhält Status und Körper des Upstream.

#### Scenario: Andere Endpunkte unverändert
- **WHEN** der Client `POST /v1/messages/count_tokens`, `GET /v1/models?limit=5`, `GET /v1/models/a%2Fb` oder einen unbekannten Pfad mit Nicht-JSON-Body aufruft
- **THEN** sieht der Upstream Methode, Pfad (inklusive Escaping), Query und Body bytegleich, und Status, Körper sowie `request-id`/`anthropic-ratelimit-*` der Antwort erreichen den Client.

#### Scenario: Upstream lehnt tollgates Zugang ab
- **WHEN** der Upstream mit 401 antwortet
- **THEN** erhält der Client 502 `api_error` „upstream authentication failed“ ohne Upstream-Körper und -Header, und der Trace trägt `upstream_auth`.

#### Scenario: Abo ohne OAuth-Zugang
- **WHEN** der Upstream auf `POST /v1/messages` mit 403 `permission_error` und `error_code: oauth_not_allowed_for_organization` antwortet
- **THEN** erhält der Client 503 `api_error` mit `Retry-After: 15` und `x-should-retry: true` ohne Upstream-Körper, tollgate loggt `upstream account not allowed` mit Fehlercode, und der Trace trägt `account_not_allowed`.

#### Scenario: Anderer 403
- **WHEN** der Upstream mit 403 ohne OAuth-Fehlercode antwortet
- **THEN** erhält der Client Status und Körper des Upstream unverändert.

#### Scenario: Abgebrochener Antwortstrom
- **WHEN** der Upstream-Strom nach dem ersten Event ohne `message_stop` abbricht
- **THEN** erhält der Client das erste Event, danach einen Verbindungsfehler statt eines regulären Endes, und der Trace trägt `stream_aborted`.

#### Scenario: Upstream-Weiterleitung
- **WHEN** der Upstream mit 302 und `Location` antwortet
- **THEN** erhält der Client diese Antwort unverändert, und der Proxy ruft die `Location` nicht ab.


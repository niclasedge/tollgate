## ADDED Requirements

### Requirement: Weiterleiten der Pipeline-Ausgabe an CLIProxyAPI

Der Proxy SHALL jede Anfrage durch den `pipeline.Runner` führen und danach den von der Pipeline gelieferten Body (nicht den Original-Body) mit derselben Methode, demselben Pfad und derselben Query an die Upstream-URL weiterleiten. Das gilt für alle Pfade, auch für `count_tokens`, Modelllisten und unbekannte Endpunkte. Antwortstatus, Antwortkörper und Antwortheader (ohne Hop-by-hop) MUST unverändert an den Client gehen; einzige Ausnahme ist ein Upstream-401, den der Proxy MUST als 502 `api_error` ohne Upstream-Körper und -Header beantworten (der Client ist zu diesem Zeitpunkt bereits authentifiziert). Bricht ein Antwortstrom nach gesendeten Headern ab, MUST der Proxy die Client-Verbindung abbrechen statt die Antwort regulär zu beenden. Der Proxy MUST Weiterleitungen des Upstream nicht folgen.

#### Scenario: Pipeline-Ausgabe wird weitergeleitet
- **WHEN** eine Stufe einen Text im Body ersetzt und der Client `POST /v1/messages?beta=true` sendet
- **THEN** erhält der Upstream `POST /v1/messages?beta=true` mit dem ersetzten Body, und der Client erhält Status und Körper des Upstream.

#### Scenario: Andere Endpunkte unverändert
- **WHEN** der Client `POST /v1/messages/count_tokens`, `GET /v1/models?limit=5`, `GET /v1/models/a%2Fb` oder einen unbekannten Pfad mit Nicht-JSON-Body aufruft
- **THEN** sieht der Upstream Methode, Pfad (inklusive Escaping), Query und Body bytegleich, und Status, Körper sowie `request-id`/`anthropic-ratelimit-*` der Antwort erreichen den Client.

#### Scenario: Upstream lehnt tollgates Zugang ab
- **WHEN** der Upstream mit 401 antwortet
- **THEN** erhält der Client 502 `api_error` „upstream authentication failed“ ohne Upstream-Körper und -Header, und der Trace trägt `upstream_auth`.

#### Scenario: Abgebrochener Antwortstrom
- **WHEN** der Upstream-Strom nach dem ersten Event ohne `message_stop` abbricht
- **THEN** erhält der Client das erste Event, danach einen Verbindungsfehler statt eines regulären Endes, und der Trace trägt `stream_aborted`.

#### Scenario: Upstream-Weiterleitung
- **WHEN** der Upstream mit 302 und `Location` antwortet
- **THEN** erhält der Client diese Antwort unverändert, und der Proxy ruft die `Location` nicht ab.

### Requirement: Header-Regeln beim Weiterleiten

Der Proxy SHALL `User-Agent`, `X-App`, `anthropic-version`, `anthropic-beta`, alle `X-Claude-Code-*`, alle `X-Stainless-*` und alle übrigen Nicht-Hop-by-hop-Header unverändert weiterreichen. `Authorization` und `x-api-key` des Clients MUST durch `Authorization: Bearer <tollgate→CPA-Key>` ersetzt werden; der Client-Key MUST NOT zum Upstream gelangen. `Accept-Encoding` MUST `identity` sein, Hop-by-hop-Header (auch die in `Connection` genannten) MUST entfernt und `Content-Length` MUST aus dem weitergeleiteten Body berechnet werden. Der Proxy MUST NOT einen `User-Agent` erfinden, den der Client nicht gesendet hat.

#### Scenario: Header-Erhalt und Ersetzung
- **WHEN** ein Client mit `Authorization`, `X-Api-Key`, `Accept-Encoding: gzip`, `Keep-Alive`, `Connection: X-Hop`, `X-Hop` und den Claude-Code-Headern sendet
- **THEN** sieht der Upstream die Claude-Code-Header unverändert, `Authorization: Bearer <CPA-Key>`, kein `X-Api-Key`, `Accept-Encoding: identity`, keinen der Hop-by-hop-Header, und in keinem Header den Client-Key.

#### Scenario: Authentifizierung per x-api-key
- **WHEN** der Client seinen Key nur als `x-api-key` sendet
- **THEN** wird er akzeptiert und der Upstream erhält nur den CPA-Key als `Authorization`.

### Requirement: SSE ohne Pufferung

Antworten mit `Content-Type: text/event-stream` SHALL Event für Event bytegetreu an den Client geschrieben und nach jedem vollständigen Event geflusht werden. Ein Event, das über mehrere Upstream-Chunks verteilt ist, MUST erst nach seiner Vollständigkeit ausgegeben werden; bereits vollständige Events MUST NOT auf spätere warten. `sse.Reader` MUST so arbeiten, dass die Aneinanderreihung aller Events die Eingabe exakt reproduziert.

#### Scenario: Erstes Event vor Stromende
- **WHEN** der Upstream ein vollständiges Event sendet, dann ein halbes, und erst nach einer Freigabe den Rest
- **THEN** liest der Client das erste Event, bevor der Upstream fertig ist, und erhält am Ende alle Events bytegleich.

#### Scenario: Chunk-Grenzen beliebig
- **WHEN** der Upstream-Strom byteweise geliefert wird
- **THEN** liefert `sse.Reader` dieselben Events wie bei einem Block, und `sse.Copy` ruft `Flush` genau einmal pro Event auf.

#### Scenario: Zu großes Event
- **WHEN** ein einzelnes Event die Obergrenze überschreitet
- **THEN** meldet `sse.Reader` `ErrEventTooLarge` und der Strom wird abgebrochen.

### Requirement: Abbruch statt ungefiltertem Weiterleiten

Bei jedem Fehler der Pipeline (Schlüssel, JSON, Stufe) SHALL der Proxy mit dem Anthropic-Fehlerkörper aus `pipeline.ErrorResponse` und `Content-Type: application/json` antworten und die Anfrage MUST NOT an den Upstream senden. Ist der Upstream nicht erreichbar, MUST der Proxy mit 502 `api_error` und fester Meldung antworten, ohne Upstream-Adresse oder Key. Zu große Bodies (über der Obergrenze) MUST mit 413 abgelehnt und nicht weitergeleitet werden.

#### Scenario: 401 ohne Weiterleitung
- **WHEN** der Key fehlt, unbekannt ist oder das Schema nicht Bearer ist
- **THEN** antwortet der Proxy mit 401 `authentication_error`, und der Upstream erhält keine Anfrage (auch nicht für `/v1/models`).

#### Scenario: Stufenfehler
- **WHEN** eine Stufe mit sensiblem Fehlertext fehlschlägt
- **THEN** antwortet der Proxy mit 500 `api_error` ohne diesen Text, und der Upstream erhält keine Anfrage.

#### Scenario: Ungültiges JSON
- **WHEN** `POST /v1/messages` einen ungültigen JSON-Body hat
- **THEN** antwortet der Proxy mit 400 `invalid_request_error` ohne Weiterleitung.

#### Scenario: Upstream nicht erreichbar
- **WHEN** der Upstream nicht antwortet
- **THEN** antwortet der Proxy mit 502 `api_error`, und die Antwort enthält weder `127.0.0.1` noch den CPA-Key.

### Requirement: Traces ohne Klartext

Der Proxy SHALL für jede authentifizierte Anfrage vor dem Forward `RecordTrace` und danach `FinishTrace` aufrufen, mit Trace-ID, Client, Sitzungsschlüssel-Hash, Konfigurationsversion, Endpunkt, Modell, Stream-Flag, Stufenmessungen (Dauer, Tokens, Hashes), Status, Dauer und Fehlerart (`stage_<name>`, `upstream_unreachable`, `client_canceled`, `stream_aborted`). Der Trace MUST NOT Body, Header, Client-Key oder CPA-Key enthalten. Anfragen ohne authentifizierten Client SHALL keinen Trace erzeugen. Fehler beim Schreiben des Traces MUST protokolliert werden und MUST NOT die Anfrage abbrechen.

#### Scenario: Erfolgreiche Anfrage
- **WHEN** eine Anfrage mit ersetzender Stufe erfolgreich weitergeleitet wird
- **THEN** enthält der gespeicherte Trace Status 200, die Stufen `intake` und die ersetzende Stufe mit zueinander passenden Hashes, und weder Klartext des Bodys noch Schlüssel.

#### Scenario: Stufenfehler
- **WHEN** eine Stufe fehlschlägt
- **THEN** hat der Trace Status 500 und die Fehlerart `stage_<name>`.

#### Scenario: Unauthentifiziert
- **WHEN** der Key ungültig ist
- **THEN** wird kein Trace geschrieben.

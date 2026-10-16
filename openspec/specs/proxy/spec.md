# proxy Specification

## Purpose
Der Proxy nimmt jede Anfrage von Claude Code an, führt sie durch die Pipeline und reicht das Ergebnis mit den Header-Regeln an CLIProxyAPI weiter; Antworten und SSE-Ströme laufen unverändert zurück.

## Requirements

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

### Requirement: Lokale Routen für Admin-API und Frontend

Der Server SHALL Anfragen auf `/api/clients` und darunter an die Admin-API und `GET`/`HEAD` auf `/`, `/index.html` und `/assets/…` an das eingebettete Frontend geben. Alle übrigen Anfragen MUST unverändert durch den Proxy laufen; der Server MUST NOT Pfade umleiten oder bereinigen.

#### Scenario: Admin-API und Frontend lokal
- **WHEN** ein Client `POST /api/clients` oder `GET /` sendet
- **THEN** beantwortet tollgate die Anfrage selbst, und der Upstream erhält keine Anfrage.

#### Scenario: Ähnliche Pfade gehen an den Proxy
- **WHEN** ein Client `GET /api/clientsX`, `GET /api/other`, `POST /` oder `GET /favicon.ico` sendet
- **THEN** läuft die Anfrage durch die Pipeline zum Upstream.

#### Scenario: Ende-zu-Ende über das Binary
- **WHEN** tollgate startet, ein Admin mit dem Token aus `admin.token_file` einen Client anlegt und der Client mit seinem Key `GET /v1/models` aufruft
- **THEN** erhält der Upstream die Anfrage mit dem tollgate→CPA-Key; nach dem Sperren des Clients wird die nächste Anfrage mit 401 abgelehnt.

### Requirement: Schlüsselprüfung vor dem Body-Read

Der Server SHALL den Client-Key anhand der Header prüfen, bevor er den Request-Body liest. Eine Anfrage ohne gültigen Key MUST eine 401-Antwort im Anthropic-Fehlerformat erhalten, ohne dass der Body gelesen oder der Upstream aufgerufen wird.

#### Scenario: Unvollständiger Body mit falschem Key
- **WHEN** ein Client `Content-Length: 1000000` ankündigt, nur wenige Bytes sendet und einen unbekannten Key nutzt
- **THEN** antwortet tollgate sofort mit 401, und der Upstream erhält keine Anfrage.

#### Scenario: Gültiger Key
- **WHEN** ein Client mit gültigem Key eine Anfrage sendet
- **THEN** liest tollgate den Body und führt die Pipeline wie bisher aus.

### Requirement: Stufenbericht im gespeicherten Trace

Der Proxy SHALL Diff und Treffer jeder Stufe in den gespeicherten Trace übernehmen. Mit eingeschalteter Redact-Stufe MUST weder der weitergeleitete Body noch die Store-Datei samt WAL noch die Logs den Klartext eines erkannten Geheimnisses enthalten.

#### Scenario: Leck-Test
- **WHEN** eine Anfrage mit E-Mail-Adresse, API-Key und Verbindungs-Passwort über den Proxy mit Redact-Stufe läuft
- **THEN** enthalten Upstream-Body, DB-Dateien und Log-Ausgabe keinen dieser Werte, und der gespeicherte Trace der Stufe `redact` hat Treffer und einen Diff mit Platzhaltern.

### Requirement: Zeit bis zum ersten Token

Der Proxy SHALL für `/v1/messages` die Zeit vom Beginn der Anfrage (derselbe Startpunkt wie `duration_us`) bis zum ersten Token als `ttft_us` im Trace speichern: bei SSE bis das erste Event `content_block_delta` (jeder Delta-Typ) an den Client geschrieben und geflusht ist, bei JSON bis die Antwort-Header des Upstreams eingetroffen sind. Bei Status ab 400, Abbruch vor dem ersten Delta, `count_tokens` und durchgereichten Endpunkten MUST `ttft_us` `NULL` bleiben. Die Messung MUST NOT die Bytes an den Client verändern oder verzögern.

#### Scenario: Verzögerter Stream
- **WHEN** ein `httptest`-Upstream `message_start` sofort und das erste `content_block_delta` erst nach 300 ms sendet
- **THEN** liegt `ttft_us` bei mindestens 300 000, höchstens bei `duration_us`, und der Client erhält den Stream byte-gleich.

#### Scenario: JSON-Antwort
- **WHEN** ein Upstream die Header einer JSON-Antwort nach 200 ms sendet
- **THEN** liegt `ttft_us` bei mindestens 200 000.

#### Scenario: Fehlerantwort
- **WHEN** der Upstream mit 429 antwortet
- **THEN** ist `ttft_us` `NULL`.

### Requirement: Rückweg-Messwerte und Urteil beim Abschluss

Der Proxy SHALL beim Abschluss einer Anfrage `return_us` (Zeit in der Rückübersetzung, ohne Warten auf den Upstream) und `return_replacements` übernehmen, sofern ein Rückweg lief, die Bytes vor und nach jeder Stufe in den Trace schreiben und danach das Urteil nach der `performance`-Spec berechnen und speichern, bevor das Live-Event zum Abschluss gesendet wird. Ein Fehler bei Abfrage oder Speicherung MUST geloggt werden und MUST NOT die Antwort an den Client beeinflussen.

#### Scenario: Live-Event mit Urteil
- **WHEN** eine Anfrage nach 20 schnellen Vergleichsanfragen künstlich verzögert abgeschlossen wird
- **THEN** trägt das abschließende `trace`-Event das Urteil `langsam`.

#### Scenario: Store-Fehler beim Urteil
- **WHEN** die Vergleichsabfrage fehlschlägt
- **THEN** hat der Client seine vollständige Antwort, und der Trace hat kein Urteil.

### Requirement: Lesefrist für den Request-Body nach Stille

tollgate SHALL das Lesen des Request-Body erst abbrechen, wenn der Client für die Dauer der Frist (30 s) keine Daten mehr schickt. Die Gesamtdauer des Uploads MUST NOT begrenzt sein, solange Daten kommen. Ein fehlgeschlagenes Lesen MUST mit Ursache, gelesenen Bytes, angekündigter Länge und Dauer geloggt werden, aber MUST NOT Inhalte des Body enthalten.

#### Scenario: Langsamer Upload über das Relay
- **WHEN** ein Client 2 MB in 45 s gleichmäßig schickt
- **THEN** liest tollgate den ganzen Body und leitet die Anfrage weiter.

#### Scenario: Client verstummt
- **WHEN** ein Client nach der Hälfte des Body 30 s nichts mehr schickt
- **THEN** bricht tollgate ab und loggt `request body read failed` mit `cause=idle_timeout`.

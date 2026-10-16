# Design

## Context

ADR-0001 (Header-Regeln), ADR-0004 (Abbruch statt ungefiltertem Weiterleiten), ADR-0008 (Bootstrap-Konfiguration, Startprüfung) und ADR-0009 (Fehlerformat, Netzwerk) legen das Verhalten fest. `pipeline.Runner` liefert Body und Header nach Stufe 0 bis 3 plus Trace; `internal/upstream` kennt nur Health. Parallel entstehen #8 (`internal/auth`), #46 (Store-Änderungen) und #9 (Frontend); dieser Change berührt nur seine eigenen Pfade.

## Goals / Non-Goals

**Goals:** Ein kleiner, korrekter Reverse-Proxy, der bytegetreu weiterleitet, was die Pipeline liefert, und der bei Fehlern fail-closed ist. Einhängepunkte für spätere Stufen und die Frontend-Routen bleiben offen.

**Non-Goals:** Siehe proposal.md („Was ausdrücklich nicht“).

## Decisions

### Eigener Forward statt `httputil.ReverseProxy`

`httputil.ReverseProxy` würde den Body aus dem Original-Request lesen; wir leiten aber `req.Body` aus der Pipeline weiter. Ein eigener, knapper Forward (`http.Client` mit Transport ohne automatische Dekompression, ohne Redirect-Folge) hält Header-Regeln, Streaming und Fehlerpfade an einer Stelle und macht sie testbar. `internal/upstream` bleibt Health-Client; die Loopback-Prüfung der URL wird über `upstream.New` wiederverwendet.

### Ablauf pro Anfrage

1. Body lesen (Obergrenze 64 MiB, darüber 413 `invalid_request_error`).
2. `Runner.Run` mit Methode, `URL.Path`, Headern und Body. Bei Fehler: Anthropic-Fehlerkörper aus `pipeline.ErrorResponse`, **kein** Forward.
3. Trace schreiben (`RecordTrace`), falls ein Client authentifiziert ist.
4. Forward: Ziel ist Upstream-Basis plus `EscapedPath` und `RawQuery` (unverändert, auch für `/v1/models/…`, `count_tokens` und unbekannte Pfade). Body ist `req.Body` der Pipeline.
5. Antwort: Status und Header weiterreichen (ohne Hop-by-hop), Body streamen.
6. `FinishTrace` mit Status, Dauer und Fehlerart.

### Header-Regeln (ADR-0001 Punkt 5)

- Entfernt: Hop-by-hop (RFC 9110, plus in `Connection` genannte), `Authorization`, `X-Api-Key`, `Content-Length` (neu berechnet), `Accept-Encoding`, `Host`.
- Gesetzt: `Authorization: Bearer <tollgate→CPA-Key>`, `Accept-Encoding: identity`.
- Alles andere bleibt unverändert, insbesondere `Anthropic-Version`, `Anthropic-Beta`, `User-Agent`, `X-App`, `X-Claude-Code-*`, `X-Stainless-*`. Fehlt der `User-Agent`, wird der Go-Standard-Agent unterdrückt statt erfunden.
- `X-Forwarded-*` wird nicht ergänzt (CLIProxyAPI soll den Client nicht anders erkennen).
- Antwortheader laufen unverändert zurück (`request-id`, `anthropic-ratelimit-*`, `retry-after`), sonst nur Hop-by-hop gefiltert. Die Transport-Dekompression ist abgeschaltet, damit kein stilles Dekodieren die Bytes ändert.

### SSE ohne Pufferung

`internal/sse.Reader` zerlegt den Strom in rohe Events (Ende durch Leerzeile, `\n` oder `\r\n`) und gibt sie bytegleich zurück; `sse.Copy` schreibt jedes vollständige Event und ruft danach `Flush`. Ein Event über Chunk-Grenzen wird gesammelt, bis es vollständig ist; es wird nie mehr als ein Event gehalten (Obergrenze 8 MiB je Event). Unvollständiger Rest am Ende wird unverändert ausgegeben. Warum Event-Framing statt reinem `io.Copy` mit Flush: der Rückweg (#14) braucht Event-Grenzen ohnehin, und die Eigenschaft „Flush pro Event“ ist so direkt prüfbar. Nicht-SSE-Antworten werden mit Flush pro Schreibvorgang kopiert.

Erkannt wird SSE am Antwort-`Content-Type: text/event-stream`.

### Fehlerpfade

| Fall | Antwort | Weitergeleitet? | Trace |
|---|---|---|---|
| Key fehlt/ungültig | 401 `authentication_error` | nein | keiner (kein Client) |
| Body kein JSON (messages) | 400 `invalid_request_error` | nein | ja, `stage_intake` |
| Stufenfehler | 500 `api_error`, feste Meldung | nein | ja, `stage_<name>` |
| Body zu groß (deklariert oder gelesen) | 413, bei `Content-Length` über der Grenze ohne den Body zu lesen | nein | keiner |
| Body kommt zu langsam (> 30 s) | 400 | nein | keiner |
| Upstream nicht erreichbar | 502 `api_error` „upstream unavailable“ (ohne Adresse, ohne Key) | – | `upstream_unreachable` |
| Client bricht ab | keine Antwort | – | Status 499, `client_canceled` |
| Upstream antwortet 401 | 502 `api_error` „upstream authentication failed“, ohne Upstream-Körper/-Header | ja | `upstream_auth` |
| Upstream-Header bleiben aus (> 10 min, wie das SDK-Timeout) | 502 `api_error` | ja | `upstream_unreachable` |
| Strom bricht mitten ab | Verbindung wird hart abgebrochen (`http.ErrAbortHandler`), damit der Client keinen abgeschnittenen Strom für vollständig hält | – | `stream_aborted` |

Warum 401 → 502: tollgate hat den Client-Key bereits geprüft. Ein 401 des Upstream heißt, dass der tollgate→CPA-Key oder das Account-Token nicht stimmt; durchgereicht würde der Client seinen eigenen Key für falsch halten, und CPA-Fehlerkörper können interne Details enthalten. 403 (`permission_error`, z. B. Modell nicht freigegeben) bleibt unverändert.

Bekannte Grenze: der Body wird vor der Schlüsselprüfung gelesen (die Pipeline prüft den Key in der Intake-Stufe). Ein Client ohne Key kann so bis zu `max_body` RAM binden; das ist im Tailnet hinnehmbar und wird als eigenes Issue verfolgt.

Ein Fehler beim Schreiben des Traces wird protokolliert und blockiert den Verkehr nicht (Messverlust ist weniger schlimm als Ausfall). Das ist **keine** Ausnahme von „Abbruch statt ungefiltert“: Redaktion und Kompression sind Stufen der Pipeline, nicht der Trace-Speicherung.

### Traces

`StageTrace` wird auf `store.Stage` abgebildet: Name, Dauer in µs, Tokens vor/nach, Hashes, Konfigurationsversion. `BytesIn/BytesOut`, `Diff` und `Matches` bleiben leer, bis die Stufen sie liefern (#13); `StageTrace` enthält keine Bytezahlen, und `internal/pipeline` gehört nicht zu diesem Change. `EstimatedInputTokens` ist `TokensAfter` der letzten Stufe. Der Sitzungsschlüssel-Hash wird aus dem Hex des Requests dekodiert. Weder Body noch Header noch Schlüssel gelangen in den Trace.

### Bootstrap-Konfiguration (`internal/config`)

Nach ADR-0008 Ebene 1, YAML über `go.yaml.in/yaml/v3` (ADR-0003). Modelliert sind nur `listen`, `upstream.{url,api_key_file}`, `db.path` und `log.level`; die übrigen ADR-Abschnitte (`management`, `crypto`, `admin`) fügen #8, #10 und #15 hinzu. Deshalb werden unbekannte Abschnitte beim Laden **ignoriert** (nicht abgelehnt); strikte Prüfung folgt mit dem vollständigen Schema. Umgebungsvariablen `TOLLGATE_<PFAD>` überschreiben die Datei über eine explizite Tabelle. Defaults: `upstream.url=http://127.0.0.1:8317`, `log.level=info`. `listen`, `upstream.api_key_file` und `db.path` sind Pflicht. Geheimnisdateien müssen Modus 0600 haben (kein Zugriff für Gruppe/Andere), dürfen nicht leer sein; Fehler nennen nie den Inhalt.

### Startprüfung der Bindung

`config.CheckListen(addr, devAllowAny)`: konkrete IP (kein Hostname, kein leerer Host, keine Wildcard `0.0.0.0`/`::`, auch nicht als IPv4-mapped), und ohne Dev-Flag innerhalb `100.64.0.0/10` oder `fd7a:115c:a1e0::/48`. Loopback ist ohne Flag **nicht** erlaubt (ADR-0008: das Flag hebt die Tailnet-Prüfung auf); Tests nutzen das Flag. `server.Listen` führt die Prüfung unmittelbar vor `net.Listen` aus, damit kein Weg am Check vorbeiführt. Die Upstream-URL prüft `upstream.New` (nur Loopback), sowohl in `Config.Validate` als auch in `server.New`.

### Übergangs-Schlüsselprüfung

`cmd/tollgate/keys.go` implementiert `pipeline.KeyVerifier` über `store.ClientByKeyHash` (SHA-256, nur Status `aktiv`). Das ist eine klar markierte Übergangslösung; #8 ersetzt sie durch `internal/auth` (RAM-Cache, Invalidierung, Konstantzeitvergleich). `internal/auth` wird hier nicht angefasst. Ein `ConfigPinner` wird nicht verdrahtet: ohne aktive Konfigurationsversion (#15) bleibt die Version 0.

### Anpassung der Gerüst-Tests in `cmd/tollgate`

`TestScaffoldHasNoNetworkDependency` beschrieb ausdrücklich das Gerüst („öffnet keine Listener“, Issue #2: „Ein dauerhafter Server würde das Bindungsverhalten aus #6 vorwegnehmen“). Mit #6 ist genau das der Auftrag, die Aussage gilt nicht mehr und der Test wird entfernt. Ersatz: `TestBinaryDoesNotDependOnThirdParty` sichert die bleibende Eigenschaft aus ADR-0001 (kein Import der CPA-Kopie).

`TestFreshCheckoutBuildsWithoutCgoAndLogsScaffoldStatus` erwartete genau eine Gerüst-Logzeile und Exit 0 ohne Konfiguration. Das Binary braucht jetzt eine Konfiguration. Der Test heißt `TestFreshCheckoutBuildsWithoutCgoAndStartsTheProxy`, baut weiterhin aus frischem Checkout mit `make build` (CGO-frei) und prüft zwei echte Vertragsfälle: (1) Wildcard-Bindung wird mit einer JSON-Fehlerzeile und Exit ≠ 0 abgelehnt, nichts auf stdout; (2) Start mit Loopback und Dev-Flag, JSON-Logzeile `tollgate: listening` mit Adresse, eine Anfrage ohne Key liefert 401, SIGTERM beendet mit Exit 0. `TestGenerateNeedsNoGlobalInstallAndIsStable` bleibt unverändert.

## Risks / Trade-offs

- Eigener Forward statt `ReverseProxy` → mehr Code, dafür volle Kontrolle und weniger Überraschungen bei Header-/Body-Behandlung; durch Tests abgesichert.
- Ganze Anfrage im Speicher (für Pipeline nötig, Obergrenze 64 MiB).
- Event-Framing könnte bei Fehlern im Upstream-Strom ein sehr langes Event puffern → Obergrenze 8 MiB, danach Abbruch des Stroms.
- Ohne Clients in der DB ist der Proxy bis #8/#10 praktisch unbenutzbar → bewusst, die Verwaltung gehört nicht hierher.
- Bei Trace-Fehlern geht Messung verloren statt Verkehr zu blockieren → protokolliert.
- Der Header `Host` wird nicht weitergereicht; Go setzt ihn auf den Upstream.

## Migration Plan

Keine Datenmigration. `tollgate.yaml` und die Key-Datei müssen vorhanden sein, sonst verweigert der Start. Revert entfernt Proxy und Verdrahtung; Gerüst-Tests kommen mit dem Revert zurück.

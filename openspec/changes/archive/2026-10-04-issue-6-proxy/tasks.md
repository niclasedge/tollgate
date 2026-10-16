# Tasks

## 1. Tests zuerst

- [x] 1.1 `internal/sse/sse_test.go`: Event-Framing (LF/CRLF, Kommentare, Rest ohne Terminator, byteweise Lieferung), Obergrenze, `Copy` mit Flush pro Event, Fuzz `FuzzReaderRoundTrip`. Rot (nicht übersetzbar), bevor `sse.go` existierte.
- [x] 1.2 `internal/config/config_test.go`: Laden, Defaults, Umgebung, Validierung, `CheckListen`-Tabelle, `ReadSecretFile`. Rot vor Umsetzung.
- [x] 1.3 `internal/server/server_test.go`: Integrationstests mit `httptest`-Mock-Upstream (Header-Regeln, Durchreichen von `count_tokens`/Modellliste/unbekannten Pfaden, Redirect, SSE über Chunk-Grenzen, 401/400/413/500/502 ohne Weiterleitung, Traces im echten Store, `Listen`). Rot vor Umsetzung.
- [x] 1.4 `cmd/tollgate`: `main_test.go` (Start verweigert, Konfiguration Pflicht, keine `third_party`-Abhängigkeit), `keys_test.go`, `contract_test.go` auf echten Start umgestellt. Rot vor Umsetzung.

## 2. Umsetzung

- [x] 2.1 `internal/sse`: `Reader`, `Copy`.
- [x] 2.2 `internal/config`: `Load`, `Validate`, `CheckListen`, `ReadSecretFile`; `go.yaml.in/yaml/v3` v3.0.4 in `go.mod`/`go.sum`.
- [x] 2.3 `internal/server`: Forward, Header-Regeln, SSE-Durchreichen, Fehlerpfade, Traces, `Listen`, `Serve`.
- [x] 2.4 `cmd/tollgate`: Verdrahtung, Übergangs-`storeKeys` (durch #8 zu ersetzen), geordnetes Beenden.

## 3. Prüfung

- [x] 3.1 `go test -race -count=1` für `./internal/server ./internal/sse ./internal/config ./cmd/tollgate` grün; `gofmt -l` leer; `go vet` für diese Pakete grün; `CGO_ENABLED=0 go build ./cmd/tollgate` grün.
- [x] 3.2 Fuzz-Lauf `FuzzReaderRoundTrip` (10 s) ohne Fund.
- [x] 3.3 `openspec validate issue-6-proxy --strict` grün.
- [ ] 3.4 Unabhängiges Review, PR mit `Closes #6`, CI grün, Merge, Archivierung (Koordinator).

## 3b. Review-Nacharbeit

- [x] 3b.1 Abgebrochener Strom bricht die Client-Verbindung ab (`TestAbortedUpstreamStreamAbortsClientConnection`).
- [x] 3b.2 `Content-Length` über der Grenze → 413 ohne Lesen, Lese-Deadline 30 s für den Body (`TestDeclaredOversizedBodyIsRejectedBeforeReading`).
- [x] 3b.3 `ResponseHeaderTimeout` 10 min im Upstream-Transport.
- [x] 3b.4 Upstream-401 → 502 `upstream_auth` (`TestUpstream401BecomesBadGateway`).
- [x] 3b.5 Tests für Antwort-Hop-by-hop, Client-Abbruch (499) und zonierte Listen-Adressen.

## 4. Offen / Folge-Issues

- [ ] 4.1 #8 ersetzt `cmd/tollgate/keys.go` durch `internal/auth`.
- [ ] 4.2 #7 liest `usage` aus der Antwort und schreibt es per `FinishTrace`; #14 setzt auf `sse.Reader` auf.
- [ ] 4.3 #15 erweitert `internal/config` (Sitzungs-Pinning, Konfigurationsversion, strikte Schemaprüfung).

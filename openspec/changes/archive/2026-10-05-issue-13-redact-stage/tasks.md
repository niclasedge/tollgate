# Tasks

## 1. Tests zuerst

- [x] 1.1 `internal/redact/stage_test.go`: Blocktabelle, Endpunkte, Nummerierung, Reservierung, Thinking/Signatur, unbekannte Typen, Determinismus, Cache-Test über drei Turns, Trace ohne Klartext, Fingerprint-Hinweis, Überlauf, Resolver-Version.
- [x] 1.2 `internal/redact/stage_fuzz_test.go`: Redact → Rehydrate ergibt die Eingabe, zweiter Lauf bytegleich.
- [x] 1.3 `internal/pipeline`: Stufenbericht im Trace, `Placeholders` nicht serialisiert, `SessionPinner` mit echtem Store.
- [x] 1.4 `internal/config`: Schalter `redact.enabled` (Standard, Umgebung, ungültiger Wert).
- [x] 1.5 `internal/server/redact_stage_test.go`: Leck-Test über Upstream, DB-Dateien und Logs; Trace mit Diff und Treffern.
- [x] 1.6 Rot sehen (Gate 2).

## 2. Umsetzung

- [x] 2.1 `pipeline`: `Match`, `Request.Report`, `Request.Placeholders`, Runner übernimmt den Bericht.
- [x] 2.2 `pipeline.SessionPinner`.
- [x] 2.3 `redact/stage.go`: `Number`, `Stage`, `Resolver`, `Builtin`, Diff, Treffer, Fingerprint-Hinweis.
- [x] 2.4 `config`: `Redact.Enabled`, `TOLLGATE_REDACT_ENABLED`.
- [x] 2.5 `server.recordTrace`: Diff und Treffer übernehmen.
- [x] 2.6 `cmd/tollgate`: Pinner und Stufe hinter dem Schalter verdrahten, Warnung beim Start.
- [x] 2.7 Verdrahtungstest `cmd/tollgate/redact_wiring_test.go` (Schalter aus: Body unverändert; an: Platzhalter und Startwarnung).

## 3. Abschluss

- [x] 3.1 `go test -race ./...`, `go vet ./...`, `make lint`, `make test`.
- [x] 3.2 `openspec validate issue-13-redact-stage --strict`.

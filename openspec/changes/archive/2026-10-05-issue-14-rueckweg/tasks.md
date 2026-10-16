# Tasks

## 1. Tests zuerst

- [x] 1.1 `internal/redact/rehydrate_test.go`: `Rehydrate`, `RehydrateJSON` (Escaping), `Holdback` (lebensfähige Präfixe, Grenze 49 Bytes, Flush), Stückelung beliebig.
- [x] 1.2 `internal/redact/rehydrate_stream_test.go`: Deltas über Grenzen, verschränkte Blöcke, Rest am Blockende, Thinking/Signatur, unbekannte Events und Felder, leere Zuordnung bytegleich, `input_json_delta` mit Escaping, Nicht-Streaming, `httptest`-Upstream mit byteweisen, zufälligen und ganzen Chunks.
- [x] 1.3 `internal/redact/rehydrate_fuzz_test.go`: Redact → Stream-Rehydrate in beliebigen Stücken ergibt das Original.
- [x] 1.4 Rot sehen (Gate 2).

## 2. Umsetzung

- [x] 2.1 `internal/redact/rehydrate.go`: `Rehydrate`, `RehydrateJSON`, `Holdback`.
- [x] 2.2 `internal/redact/rehydrate_stream.go`: `CopyRehydrated`, `RehydrateBody`.
- [x] 2.3 Verdrahtungsanleitung im Bericht an den Koordinator (server.go, main.go).

## 3. Abschluss

- [x] 3.1 `go test -race ./internal/redact/... ./internal/server/...`, `go vet`, `gofmt`.
- [x] 3.2 `openspec validate issue-14-rueckweg --strict`.

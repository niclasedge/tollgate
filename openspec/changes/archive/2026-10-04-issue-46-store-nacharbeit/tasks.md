# Tasks

## 1. Tests zuerst

- [x] 1.1 Tests für Sidecar-Rechte, Query-Plan (sechs Kombinationen) und paralleles Öffnen schreiben; rot bestätigt.
- [x] 1.2 Toten Code in `review_test.go` entfernen.

## 2. Umsetzung

- [x] 2.1 `ensureFile` für `-wal`/`-shm`.
- [x] 2.2 Migration `0002_trace_indexes.sql` und `listTracesQuery`.
- [x] 2.3 `pingRetryBusy` im `Open`.

## 3. Prüfung

- [x] 3.1 `go test -race -count=1 ./internal/store/...`, `CGO_ENABLED=0 go test`, `gofmt -l`, `go vet`, `CGO_ENABLED=0 go build ./cmd/tollgate`.
- [x] 3.2 `openspec validate issue-46-store-nacharbeit --strict`.

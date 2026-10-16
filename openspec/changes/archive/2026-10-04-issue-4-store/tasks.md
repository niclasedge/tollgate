# Tasks

## 1. Tests zuerst

- [x] 1.1 Tabellengetriebene Tests für Migrationen, Clients, Konfigurationsversionen, Sitzungen, Traces, Feld-Allowlist und Leck-Test schreiben; rot ausführen (Paket existiert noch nicht).

## 2. Umsetzung

- [x] 2.1 `modernc.org/sqlite` v1.60.1 in `go.mod` aufnehmen.
- [x] 2.2 Migrations-Runner und `0001_init.sql`.
- [x] 2.3 Store-API (Open/Close, Clients, Konfigurationsversionen, Sitzungen, Traces).

## 3. Review-Nacharbeit

- [x] 3.1 Tests zuerst (rot bestätigt), dann Fixes: Modellname mit Klammern und Normalisierung zu `unknown`, Dateirechte 0600, Keyset-Pagination und dynamischer Client-Filter, Trigger gegen `INSERT OR REPLACE`, `_txlock=immediate` und Migration in einer IMMEDIATE-Transaktion, Hinweis an `Seal`.
- [x] 3.2 Nits: ms-gekürzte `CreatedAt`, Bereichsprüfungen (`StartedAt`, negative Werte, Status 100 bis 599), `PinSession`-Validierung, Obergrenze für `ListConfigVersions`.
- [x] 3.3 Zusatztests: Purge-Grenzen, `MaxDiffBytes` bei Konfigurationsversionen, Listenlimit, Stufenreihenfolge nach `seq`, 50-Goroutinen-Test, zwei Stores mit 40 parallelen Versionen, Leck-Test mit Positivkontrolle.

## 4. Prüfung

- [x] 4.1 `go test -race -count=1 ./internal/store/...`, `CGO_ENABLED=0 go test ./internal/store/...`, `gofmt -l`, `go vet`, `CGO_ENABLED=0 go build ./cmd/tollgate`.
- [x] 4.2 `openspec validate issue-4-store --strict`.

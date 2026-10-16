# Proposal

Issue: [#4 Store: SQLite (ohne cgo) für Traces, Messwerte, Konfigurationsversionen](https://github.com/niclasedge/tollgate/issues/4)

Geht zurück auf GitHub-Issue #4. Abhängigkeit #2 ist erfüllt.

## Why

Pipeline (#5), Proxy (#6), Meter (#7), Client-Keys (#8) und Konfiguration (#15) brauchen einen gemeinsamen, persistenten Speicher. ADR-0003, -0006, -0008 und -0009 legen SQLite ohne cgo, den eigenen Migrations-Runner und die Tabellen fest. Die harte Regel „in der DB nur redigierte Daten“ muss im Store-API selbst durchgesetzt und getestet sein.

## What Changes

- `internal/store`: Öffnen einer SQLite-Datei (`modernc.org/sqlite`, WAL, ein Schreiber, getrennter Lese-Pool), eigener Migrations-Runner mit eingebetteten SQL-Dateien und Versionstabelle.
- Schema `0001`: `clients`, `config_versions` (unveränderlich), `config_active`, `sessions`, `traces`, `trace_stages`.
- Typisiertes Go-API für Clients (nur Hashes), Konfigurationsversionen (Wortliste nur als opaker, bereits versiegelter Blob), Sitzungs-Pinning mit Aufräumen, Traces mit Stufen-Messwerten.
- Tests: tabellengetrieben, Idempotenz und Determinismus der Migrationen, Feld-Allowlist gegen Klartextfelder, Leck-Test über die DB-Datei, `CGO_ENABLED=0`.

## Capabilities

### New Capabilities

- `store`: Persistenz von Clients, Konfigurationsversionen, Sitzungen und Traces ohne Klartext.

### Modified Capabilities

Keine.

## Impact

- Neue Laufzeitabhängigkeit `modernc.org/sqlite` v1.60.1 (ADR-0003); `go get` hebt transitiv `golang.org/x/tools` an. `go.mod`/`go.sum` ändern sich.
- Der Store wird noch nicht in `cmd/tollgate` verdrahtet (kommt mit #6/#8).

## Was ausdrücklich nicht

Keine Verdrahtung in `main`, keine HTTP-API, keine Verschlüsselung (nur Aufnahme eines fertigen Chiffrats, #15), keine Schlüsselerzeugung (#8), keine Aggregationsabfragen für Kosten (#7/#10), keine Rotations-Übergangszeit für alte Client-Keys (ADR-0009: Standard 0).

## Berührte Dateien

- `go.mod`, `go.sum`
- `internal/store/store.go`, `migrate.go`, `clients.go`, `config.go`, `sessions.go`, `traces.go`, `errors.go`
- `internal/store/migrations/0001_init.sql` (ersetzt `.gitkeep`)
- `internal/store/*_test.go`
- `openspec/changes/issue-4-store/**`

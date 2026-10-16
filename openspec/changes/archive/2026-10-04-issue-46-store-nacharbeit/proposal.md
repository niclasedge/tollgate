# Proposal

Issue: [#46 Store-Nacharbeit: Sidecar-Rechte, Query-Plan der Keyset-Abfrage, parallele Migration testen](https://github.com/niclasedge/tollgate/issues/46)

Geht zurück auf die Nachprüfung des Reviews von #4 (PR #45). Es sind Nits, die aber beim Test eine echte Lücke aufdecken (siehe design.md, paralleles Öffnen).

## Why

- Eine alte 0644-Installation hinterlässt `-wal`/`-shm` mit offenen Rechten; `ensureFile` setzt nur die Hauptdatei.
- Die Keyset-Abfrage `ORDER BY started_at DESC, id DESC` nutzt mit den Indizes aus 0001 einen Temp-Sort (`USE TEMP B-TREE`).
- Der Test für zwei gleichzeitig öffnende Stores fehlt; er zeigt, dass `Open` auf einer frischen Datei mit `SQLITE_BUSY` scheitern kann.
- Toter Code in `review_test.go`.

## What Changes

- `ensureFile` setzt vorhandene `-wal`/`-shm` auf 0600.
- Neue Migration `0002_trace_indexes.sql`: `traces_started_at (started_at DESC, id DESC)` und `traces_client (client_id, started_at DESC, id DESC)`. 0001 bleibt unverändert (stabile Prüfsumme).
- `Open` wiederholt den ersten Verbindungsaufbau bei `SQLITE_BUSY` (kurzes Backoff, höchstens 5 s).
- Query-Aufbau von `ListTraces` in `listTracesQuery` ausgelagert, damit der Plan-Test die echte Abfrage prüft.
- Tests: Sidecar-Rechte, Query-Plan ohne Temp-Sort (mit/ohne Client, mit/ohne Cursor), paralleles Öffnen. Toter Code entfernt.

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `store`: zusätzliche Anforderungen (Sidecar-Rechte, Trace-Liste ohne Temp-Sort, paralleles Öffnen).

## Impact

Schema-Version 2; bestehende Datenbanken erhalten die neuen Indizes beim nächsten `Open`. Keine API-Änderung.

## Was ausdrücklich nicht

Keine neuen Store-Abfragen (z. B. Client-Key-Lookups für #8), keine Änderung an 0001, keine Änderung der öffentlichen API.

## Berührte Dateien

- `internal/store/store.go`, `traces.go`, `review_test.go`
- `internal/store/migrations/0002_trace_indexes.sql`
- `openspec/changes/issue-46-store-nacharbeit/**`

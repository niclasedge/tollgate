# Design

## Decisions

- **Sidecar-Rechte:** `ensureFile` prüft `-wal` und `-shm` (falls vorhanden) und setzt sie auf 0600, nie lockerer. Nicht vorhandene Sidecars werden übergangen; SQLite legt neue mit dem Modus der Hauptdatei an.
- **Index statt Abfrageumbau:** SQLite kann `(started_at DESC, id ASC)` weder vorwärts noch rückwärts für `started_at DESC, id DESC` lesen und sortiert zusätzlich (`USE TEMP B-TREE FOR LAST TERM OF ORDER BY`). Die Indizes werden daher mit `id DESC` neu angelegt; `traces_client` bekommt `id` als dritte Spalte. Die Umstellung läuft in Migration 0002 (DROP + CREATE), damit die Prüfsumme von 0001 stabil bleibt, obwohl es noch keinen Release gibt.
- **Plan-Test:** `listTracesQuery` liefert SQL und Argumente der echten Abfrage; der Test führt `EXPLAIN QUERY PLAN` für sechs Filterkombinationen aus und verlangt keinen `TEMP B-TREE` und den erwarteten Index. Der OR-Cursor (`started_at < ? OR (started_at = ? AND id < ?)`) wurde mit dem neuen Index geprüft; der Test belegt das Ergebnis.
- **Paralleles Öffnen:** Der Test deckte auf, dass `PRAGMA journal_mode(WAL)` auf einer frischen Datei bei mehreren gleichzeitigen Openern sofort `SQLITE_BUSY` liefern kann, ohne `busy_timeout` abzuwarten. Die Migration selbst ist durch `BEGIN IMMEDIATE` serialisiert. Fix: `pingRetryBusy` wiederholt den ersten Ping des Schreibers mit Backoff (5 ms, verdoppelt bis 100 ms, höchstens 5 s oder Ende des Kontexts). Andere Fehler werden sofort zurückgegeben.

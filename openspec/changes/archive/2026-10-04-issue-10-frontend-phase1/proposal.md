# Proposal

Issue: [#10 Frontend Phase 1: Live, Kosten, Clients, Accounts](https://github.com/niclasedge/tollgate/issues/10)

Geht zurück auf GitHub-Issue #10. Abhängigkeiten #7, #8 und #9 sind erfüllt.

## Why

tollgate misst jede Anfrage (Traces, Usage, Kosten), aber das Frontend zeigt nur leere Bereiche. Wer tollgate betreibt, sieht weder laufende Anfragen noch Kosten, kann Clients nur per `just client` anlegen und weiß nicht, welcher Account aktiv ist.

## What Changes

- Neue Dashboard-API unter `/api/`, geschützt mit dem Admin-Token:
  - `GET /api/traces`: Live-Liste, seitenweise
  - `GET /api/traces/events`: SSE, ein Event pro gestartetem und abgeschlossenem Trace
  - `GET /api/costs`: Tokens und Kosten pro Tag, Client und Modell
  - `GET /api/status`: Health von tollgate und CLIProxyAPI, Accounts aus der Management-API
- Der Server routet `/api/` und alles darunter an die Admin-Seite (bisher nur `/api/clients`).
- tollgate liest das Management-Secret von CLIProxyAPI aus `upstream.management_key_file` (Env `TOLLGATE_UPSTREAM_MANAGEMENT_KEY_FILE`). Ohne Datei zeigt der Status „Accounts nicht verfügbar“. Der Container bekommt das Secret gemountet.
- Frontend:
  - Anmeldung mit dem Admin-Token, lokal gespeichert, Abmelden möglich.
  - Bereiche Live, Kosten, Clients (erzeugen, sperren, rotieren, Setup-Snippet) und Accounts.
  - Die TS-Typen werden per tygo generiert.

## Capabilities

### New Capabilities

- `dashboard-api`: Traces, Live-Events, Kosten und Status für das Frontend.
- `frontend-views`: die vier Bereiche der Phase 1.

### Modified Capabilities

Keine. Das Routing von `/api/` ist Teil von `dashboard-api`.

## Was ausdrücklich nicht

- Keine Detailansicht mit Stufen-Diffs: das ist #16.
- Kein Account-Wechsel und keine Abo-Verwaltung: das ist #28/#29.
- Keine Diagramm-Bibliothek; Balken entstehen mit CSS.

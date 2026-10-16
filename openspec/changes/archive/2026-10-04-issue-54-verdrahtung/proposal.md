# Proposal

Issue: [#54 Verdrahtung: internal/auth, Admin-API und Frontend im Binary](https://github.com/niclasedge/tollgate/issues/54)

Geht zurück auf GitHub-Issue #54. Abhängigkeiten #6, #8 und #9 sind erfüllt.

## Why

Proxy (#6), Client-Keys mit Admin-API (#8) und Frontend-Shell (#9) sind gemergt, aber nicht verbunden. Das Binary prüft Keys noch mit dem Übergangs-Verifier aus #6, die Admin-API ist nicht erreichbar, und das Frontend wird nicht ausgeliefert. Ohne Verdrahtung kann niemand einen Client-Key anlegen und Claude Code über tollgate nutzen.

## What Changes

- `cmd/tollgate` nutzt `auth.NewVerifier(store)` statt `cmd/tollgate/keys.go`; die Übergangslösung entfällt.
- Der Server routet `/api/clients` und darunter an die Admin-API, `GET`/`HEAD` von `/`, `/index.html` und `/assets/…` an das eingebettete Frontend und alles andere an den Proxy.
- Bootstrap-Konfiguration: `admin.token_file` (Env `TOLLGATE_ADMIN_TOKEN_FILE`), Default `admin.token` neben `db.path`; fehlt die Datei, wird sie beim Start mit Modus 0600 erzeugt.
- `tygo.yaml` generiert die TS-Typen der Admin-API nach `web/src/lib/types/auth.ts`.

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `proxy`: lokale Routen für Admin-API und Frontend vor dem Durchreichen.
- `bootstrap-config`: Admin-Token-Datei.

## Was ausdrücklich nicht

- Keine Fachansichten im Frontend (Clients, Live, Kosten): das ist #10.
- Keine Prüfung des Client-Keys vor dem Body-Read: das ist #52.

# Proposal

Issue: [#8 Client-Keys: erzeugen, sperren, rotieren (Backend)](https://github.com/niclasedge/tollgate/issues/8)

## Why

Stufe 0 der Pipeline (#5) prüft Client-Keys nur über das Interface `pipeline.KeyVerifier`, und der Store (#4) hält die Tabelle `clients`. Es fehlen die Key-Erzeugung, eine echte Prüfung, die Sperren und Rotation sofort durchsetzt, und eine abgesicherte Admin-API für das Frontend (#10).

## What Changes

- Key-Erzeugung `tg_` plus 32 Byte aus `crypto/rand` (base64url), Speicherung nur als SHA-256 samt Anzeige-Hinweis.
- `auth.Verifier` implementiert `pipeline.KeyVerifier` ohne Cache: jeder Request liest die aktuelle Zeile.
- Admin-API als `http.Handler` (`auth.NewAdminAPI`) unter `/api/clients`: anlegen, auflisten, sperren, rotieren.
- Admin-Token-Middleware (Bearer, konstante Zeit) und Laden/Erzeugen der Token-Datei.
- JSON-Typen als Go-Structs in `internal/auth/types.go`, für spätere tygo-Generierung.

## Capabilities

### New Capabilities

- `client-auth`: Client-Key-Lebenszyklus, Key-Prüfung und Admin-Zugang.

### Modified Capabilities

Keine.

## Impact

- Nur Paket `internal/auth` und dieser Change. Keine neue Abhängigkeit (Standardbibliothek).
- Verdrahtung in `internal/server`/`cmd` und die tygo-Paketliste (`tygo.yaml`) macht der Koordinator nach dem Merge.

## Was ausdrücklich nicht

Keine Verdrahtung in den Server, kein Frontend, keine Änderung am Store, keine Entsperr-Route, keine Übergangszeit für rotierte Keys (Standard 0 laut ADR-0009), keine Rate-Limits, keine Verschlüsselung der Wortliste (#15).

## Berührte Dateien

- `internal/auth/doc.go`, `key.go`, `verifier.go`, `adminapi.go`, `admintoken.go`, `types.go`, `auth_test.go`
- `openspec/changes/issue-8-client-keys/**`

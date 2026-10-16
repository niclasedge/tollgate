# Proposal

Issue: [#52 Client-Key vor dem Body-Read prüfen](https://github.com/niclasedge/tollgate/issues/52)

Geht zurück auf GitHub-Issue #52 (aus dem Review von #6). Abhängigkeit #6 ist erfüllt.

## Why

Der Proxy liest den Request-Body vollständig (bis `max_body`, Default 64 MiB), bevor die Intake-Stufe den Client-Key prüft. Ein Client ohne gültigen Key kann so pro Anfrage bis zu 64 MiB RAM binden.

## What Changes

- `Intake.Authenticate(ctx, header)` prüft den Key nur anhand der Header. `Process` nutzt dieselbe Funktion.
- `Runner.Authenticate` ruft die erste Stufe mit dieser Fähigkeit auf (Interface `Authenticator`).
- Der Server prüft den Key vor dem Body-Read. Eine unauthentifizierte Anfrage erhält 401, ohne dass der Body gelesen wird, und hinterlässt wie bisher keinen Trace.

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `proxy`: Schlüsselprüfung vor dem Body-Read.

## Was ausdrücklich nicht

- Keine Rate-Limits pro Client oder IP.
- Keine Änderung an der Key-Prüfung selbst (ADR-0009).

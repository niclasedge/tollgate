# Proposal

Issue: [#68 Deploy ohne Tailscale-Sidecar: Port auf die Tailnet-IP des Hosts veröffentlichen](https://github.com/niclasedge/tollgate/issues/68)

Geht zurück auf GitHub-Issue #68. Keine offenen Abhängigkeiten.

## Why

Der Stack läuft auf einem Rechner, der selbst im Tailnet ist. Der Tailscale-Sidecar aus ADR-0012 macht daraus ein zweites Tailnet-Gerät, verlangt einen Auth-Key und Kernel-Rechte (`NET_ADMIN`, `/dev/net/tun`), nur damit tollgate im Container eine Tailnet-IP binden kann.

## What Changes

- Der Sidecar entfällt. Docker veröffentlicht tollgate nur auf der Tailnet-IP des Hosts (`<tailnet-ip>:8443`).
- tollgate bekommt den Konfigurationswert `published`: die Host-Adresse, unter der Docker den Port veröffentlicht. Ist er gesetzt, MUST er in den Tailnet-Bereichen liegen, und `listen` darf nur eine konkrete private Container-Adresse sein.
- `just setup` ermittelt die Tailnet-IP des Hosts (`tailscale ip -4`, unter macOS auch die App), schreibt sie nach `.env` und fragt keinen Auth-Key mehr ab.
- CLIProxyAPI hält den gemeinsamen Netzwerk-Namespace und bindet weiter nur `127.0.0.1`.
- ADR-0013 löst ADR-0012 in diesem Punkt ab.

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `deployment`: ohne Sidecar, Veröffentlichung auf der Host-Tailnet-IP, kein Auth-Key.
- `bootstrap-config`: neuer Wert `published` und angepasste Startprüfung.

## Was ausdrücklich nicht

- Keine Variante mit Sidecar für Server ohne Tailscale auf dem Host (bei Bedarf eigenes Issue).
- Kein Erkennen einer geänderten Host-Tailnet-IP im laufenden Betrieb: `just setup` oder `just up` schreibt sie neu.

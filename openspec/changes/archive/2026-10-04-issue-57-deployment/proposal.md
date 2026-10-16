# Proposal

Issue: [#57 Deployment: just setup, alles im Container](https://github.com/niclasedge/tollgate/issues/57)

Geht zurück auf GitHub-Issue #57. Keine offenen Abhängigkeiten.

## Why

Der erste echte Test lief mit Hand-Schritten auf dem Host: Config rendern, CLIProxyAPI mit `go run` starten, tollgate-Binary starten. Das ist fehleranfällig und erfüllt die Bindungsregel nur mit Dev-Flag. Die App soll mit einem Befehl eingerichtet werden und vollständig in Containern laufen.

## What Changes

- `just setup` prüft Werkzeuge, legt fehlende Secrets an, fragt den Tailscale-Auth-Key ab, startet bei fehlendem Token den Claude-Login im Container und deployt den Stack mit Health-Warten.
- tollgate bekommt ein Image (Frontend + Binary) und einen Entrypoint, der die Tailnet-IP ermittelt, Secrets mit 0600 bereitstellt und ohne Root startet.
- `docker-compose.yml` enthält Tailscale-Sidecar, CLIProxyAPI, tollgate und einen Login-Dienst; tollgate und CLIProxyAPI teilen den Namespace des Sidecars (ADR-0012).
- `just login`, `just import-token`, `just client`, `just status`, `just up/down/logs`, `just smoke`.
- Das `justfile` wird versioniert; lokales ops-lab-Werkzeug per `import? 'tools/ops.just'`.

## Capabilities

### New Capabilities

- `deployment`: Einrichtung und Betrieb des Container-Stacks per `just`.

### Modified Capabilities

Keine.

## Was ausdrücklich nicht

- Kein Account-Wechsel und keine Verwaltung mehrerer Tokens: das ist #28/#29.
- Keine CI-Ausführung des Smoke-Tests (baut drei Images und braucht Docker; folgt bei Bedarf als eigenes Issue).

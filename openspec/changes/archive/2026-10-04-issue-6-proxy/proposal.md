# Proposal

Issue: [#6 Proxy: Forward und Durchreichen aller Endpunkte inkl. SSE](https://github.com/niclasedge/tollgate/issues/6)

Geht zurück auf GitHub-Issue #6. Abhängigkeiten #3 und #5 sind erfüllt.

## Why

Pipeline-Kern (#5), Store (#4) und CLIProxyAPI-Anbindung (#3) existieren, aber nichts nimmt Anfragen entgegen. Ohne Proxy kann Claude Code nicht über tollgate arbeiten, und die Folge-Issues (Meter #7, Redact #13, Rückweg #14) haben keinen Einhängepunkt.

## What Changes

- HTTP-Proxy in `internal/server`: Anfrage durch den `pipeline.Runner`, danach Stufe 4 Forward an CLIProxyAPI. Alle Pfade werden weitergeleitet, nicht nur `/v1/messages`.
- Header-Regeln nach ADR-0001 Punkt 5: Durchreichen, Client-Key durch den tollgate→CPA-Key ersetzen, `Accept-Encoding: identity`, Hop-by-hop entfernen, `Content-Length` neu berechnen.
- SSE-Antworten werden ohne Pufferung, mit Flush pro Event, durchgereicht (`internal/sse`: Event-Framing, bytegetreu).
- Bei Pipeline-Fehlern (401, 400, 500) wird nicht weitergeleitet; der Client erhält den Anthropic-Fehlerkörper.
- Startprüfung der Bindung nach ADR-0008: konkrete Tailnet-IP, keine Wildcard, Upstream nur Loopback; Flag `--dev-allow-any-listen` für Entwicklung und Tests.
- Minimale Bootstrap-Konfiguration (`internal/config`) und Verdrahtung in `cmd/tollgate`: Konfiguration laden, Store öffnen, Runner mit Intake bauen, Server starten, bei SIGTERM geordnet beenden.
- Traces werden über `RecordTrace`/`FinishTrace` geschrieben (nur Hashes, Zähler, Metadaten).
- Der Gerüst-Vertragstest in `cmd/tollgate` wird auf den echten Start umgestellt (siehe design.md).

## Capabilities

### New Capabilities

- `proxy`: Weiterleiten aller Endpunkte an CLIProxyAPI mit Header-Regeln, SSE-Durchreichen, Abbruch bei Pipeline-Fehlern und Traces.
- `bootstrap-config`: Bootstrap-Konfiguration (ADR-0008 Ebene 1), Startprüfung der Bindung, Geheimnisdateien.

### Modified Capabilities

Keine. `pipeline-core` und `cliproxyapi-integration` werden genutzt, nicht geändert; die Übergangs-Schlüsselprüfung in `cmd/tollgate` ist kein Spec-Bestandteil und wird durch #8 ersetzt.

## Impact

- Neue Laufzeitabhängigkeit `go.yaml.in/yaml/v3` v3.0.4 (ADR-0003, YAML).
- `cmd/tollgate` öffnet jetzt Netzwerkverbindungen und eine Datenbank; der Gerüst-Test „keine Netzwerkabhängigkeit“ entfällt.
- Bis #8 und #10 Client-Keys anlegen können, lehnt der Proxy jede Anfrage mit 401 ab (leere `clients`-Tabelle).

## Was ausdrücklich nicht

Keine Redact-, Compress- oder Meter-Logik, kein Rückweg (Rehydrate), keine Usage-Auswertung der Antwort, keine Frontend- oder `/api/*`-Routen, keine echte Client-Key-Verwaltung (#8), kein Konfigurations-Hot-Reload und kein Sitzungs-Pinning (#15), kein Management-API-Client für den aktiven Account (#10), keine Änderungen an `internal/store`, `internal/pipeline`, `internal/upstream`, `Makefile`, `.github/`.

## Berührte Dateien

- `go.mod`, `go.sum`
- `cmd/tollgate/main.go`, `cmd/tollgate/keys.go`, `cmd/tollgate/keys_test.go`, `cmd/tollgate/main_test.go`, `cmd/tollgate/contract_test.go`
- `internal/config/doc.go`, `internal/config/config.go`, `internal/config/config_test.go`
- `internal/server/doc.go`, `internal/server/server.go`, `internal/server/server_test.go`
- `internal/sse/doc.go`, `internal/sse/sse.go`, `internal/sse/sse_test.go`
- `openspec/changes/issue-6-proxy/**`

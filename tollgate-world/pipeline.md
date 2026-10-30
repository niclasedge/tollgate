# Pipeline · Proxy

## Auf einen Blick

Jede Anfrage von Claude Code läuft durch tollgate, dann über CLIProxyAPI zu Anthropic, und die Antwort rückwärts. Die Pipeline besteht aus Stufen hinter einem gemeinsamen `Stage`-Interface: Eingang, Redact, Compress, Meter, Forward, Rückweg. Jede Stufe hinterlässt einen Trace.

## Datenfluss

Claude Code → tollgate (Go) → CLIProxyAPI (localhost) → Anthropic
Die Antwort läuft rückwärts durch tollgate.

Alle anderen Endpunkte, die Claude Code aufruft (z. B. `count_tokens`, Modelllisten),
werden unverändert durchgereicht. Header wie `anthropic-version` und `anthropic-beta`
bleiben erhalten.

Belege: ADR-0001 · ADR-0013

Wo: `docs/ARCHITECTURE.md`, `internal/server/`, `internal/upstream/`

## Stufen

Pipeline-Stufen, jede als eigene Implementierung eines gemeinsamen `Stage`-Interfaces:
0. **Eingang:** Client-Key prüfen, `/v1/messages` parsen, Trace-ID vergeben.
1. **Redact:** Geheimnisse durch nummerierte, deterministische Platzhalter ersetzen,
   z. B. `<redacted:email_1>`. Die Zuordnung liegt nur im RAM, mit TTL pro Sitzung.
2. **Compress:** Nur `tool_result` von Bash-Aufrufen, mit einer Regel pro Befehl
   (`cargo test`, `go test`, `git log`, `npm install` usw.). Den Befehl liest du aus dem
   zugehörigen `tool_use` im Verlauf. Das ersetzt rtk vollständig.
3. **Meter:** Vorab eine Schätzung loggen, nach der Antwort das `usage`-Feld
   (`input`, `output`, `cache_read`, `cache_creation`) speichern.
4. **Forward:** Weiterleiten an CLIProxyAPI auf `localhost`.
5. **Rückweg:** Platzhalter beim Streamen zurückübersetzen, auch in `input_json_delta`.
   Puffern, bis ein Platzhalter vollständig ist.

Belege: ADR-0004 · ADR-0006 · ADR-0007

Wo: `internal/pipeline/`, `internal/redact/`, `internal/compress/`, `internal/meter/`, `internal/upstream/`, `internal/sse/`, `internal/jsonsplice/`

## Technik

- SQLite ohne cgo (z. B. `modernc.org/sqlite`), damit tollgate ein statisches Binary bleibt.
- Frontend per `embed` ins Binary.
- Logging mit `log/slog`, Tracing optional mit OpenTelemetry.

Die Paketstruktur steht in ADR-0002 und `docs/ARCHITECTURE.md` (Komponenten und Pakete).

Belege: ADR-0002 · ADR-0003

Wo: `cmd/tollgate/`, `internal/store/`, `web/embed.go`, `go.mod`

## Nachvollziehbarkeit

- Jede Anfrage bekommt eine Trace-ID. Pro Stufe speicherst du: Dauer, Tokens
  vorher/nachher, Hash von Ein- und Ausgabe, einen Diff der redigierten Fassung und
  die verwendete Konfigurationsversion.
- Das Frontend zeigt pro Anfrage eine Timeline aller Stufen mit Vorher/Nachher-Diff.

Belege: ADR-0004

Wo: `internal/store/`, `internal/dashboard/`, `web/src/lib/components/Timeline.svelte`

# Proposal

Issue: [#14 Rückweg: Platzhalter im Stream zurückübersetzen](https://github.com/niclasedge/tollgate/issues/14)

Depends on #13 (Redact-Stufe, `Request.Placeholders`) und #6 (Proxy, SSE-Framing in
`internal/sse`). Verbindlich ist [ADR-0006](../../../docs/adr/0006-platzhalter-sitzungen-rueckweg.md),
Abschnitt „Rückweg im Stream“ und Blocktabelle, ergänzt um ADR-0004 (Escaping, Byte-Splicing).

## Why

Seit #13 ersetzt tollgate Geheimnisse durch `<redacted:TYP_N>`. Ohne Rückweg sieht Claude Code
diese Platzhalter in Antworten und in `tool_use`-Argumenten; ein Bash-Befehl mit
`<redacted:email_1>` liefe mit dem Platzhalter statt mit dem echten Wert. Deshalb bleibt
`redact.enabled` bis zu diesem Change aus.

## What Changes

- `redact.Rehydrate` / `redact.RehydrateJSON`: ersetzen bekannte Platzhalter durch den
  Klartext (bzw. den JSON-escapten Klartext). Unbekannte Platzhalter bleiben stehen.
- `redact.Holdback`: Puffer pro Content-Block, der einen unvollständigen Platzhalter am
  Ende eines Deltas zurückhält (höchstens 49 Bytes) und erst mit dem nächsten Delta oder am
  Blockende ausgibt.
- `redact.CopyRehydrated`: SSE-Rückweg. Schreibt `text_delta` und `input_json_delta` per
  Byte-Splicing des einen Feldes um; Thinking, Signaturen, alle übrigen Events und Felder
  bleiben bytegleich.
- `redact.RehydrateBody`: Nicht-Streaming-Antwort, `text` und String-Werte in
  `tool_use.input` per Byte-Splicing.
- Die Verdrahtung in `internal/server/server.go` (`forward`) folgt in einem eigenen Commit
  des Koordinators; die Anleitung steht in `design.md`.

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `redaction`: Rückweg im Stream und in Nicht-Streaming-Antworten.

## Impact

Branch `issue-14/rueckweg`, gemeinsamer Workspace. Nur neue Dateien:
`internal/redact/rehydrate.go`, `internal/redact/rehydrate_stream.go` und Tests. Keine
Migration, keine neue Abhängigkeit.

## Was ausdrücklich nicht

- Keine Rückübersetzung von `thinking_delta`, `signature_delta` oder Thinking-Blöcken.
- Kein Cache-Test über echte Turns (#17).
- Keine Verdrahtung in `server.go`/`main.go` in diesem Change-Teil (fremde, uncommittete
  Arbeit in diesen Dateien); das Einschalten von `redact.enabled` als Standard bleibt eine
  eigene Entscheidung.

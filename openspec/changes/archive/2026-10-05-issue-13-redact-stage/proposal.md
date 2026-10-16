# Proposal

Issue: [#13 Redact-Stage: nummerierte deterministische Platzhalter](https://github.com/niclasedge/tollgate/issues/13)

Geht zurück auf GitHub-Issue #13. Die Abhängigkeiten #5, #12 und #41 sind geschlossen.
Verbindlich ist [ADR-0006](../../../docs/adr/0006-platzhalter-sitzungen-rueckweg.md)
(Grammatik, Nummerierung pro Anfrage, Blocktabelle, `count_tokens`, Sitzungs-Pinning,
Spike `cc_version`), ergänzt um ADR-0004 (Byte-Splicing) und ADR-0009 (kein Klartext).

## Why

Der Go-Port aus #12 erkennt Geheimnisse, ist aber noch nicht in der Pipeline. Ohne
Redact-Stufe gehen Anfragen von Claude Code im Klartext an CLIProxyAPI. #13 baut die Stufe,
die Geheimnisse vor dem Weiterleiten durch nummerierte, deterministische Platzhalter ersetzt
und die Zuordnung für den Rückweg (#14) im RAM bereitlegt.

## What Changes

- Neue Stufe `redact` (`internal/redact/stage.go`) für `POST /v1/messages` und
  `POST /v1/messages/count_tokens`: Platzhalter `<redacted:TYP_N>` nach ADR-0006, gezählt
  pro Anfrage in Dokumentreihenfolge, reservierte Nummern werden übersprungen.
- Nur die Blöcke der ADR-0006-Tabelle werden berührt (Ziele aus `jsonsplice.Scan`):
  User- und Assistant-Text, `tool_result`, `tool_use.input`. Thinking, Signaturen,
  `system`, `tools`, `metadata` und unbekannte Blocktypen bleiben Byte für Byte erhalten.
- Zuordnung `Platzhalter → Wert` nur im RAM der Anfrage (`Request.Placeholders`), nie in
  Trace, Log oder DB.
- Trace der Stufe: zusätzlich ein redigierter Diff und Treffer (Offset, Länge, Typ, Regel);
  der Proxy speichert beides im vorhandenen Store-Feld.
- Feste eingebaute Konfiguration `builtin-v1` (`redact.DefaultOptions()`) hinter einem
  `Resolver`, in den #15 die versionierte Konfiguration einhängt.
- Sitzungs-Pinning: `pipeline.SessionPinner` pinnt die aktive Konfigurationsversion je
  Sitzung im Store (ADR-0006); ohne aktive Version gilt `builtin-v1` (Version 0).
- Bootstrap-Schalter `redact.enabled` / `TOLLGATE_REDACT_ENABLED`, Standard **aus**, bis der
  Rückweg (#14) gemergt ist.
- Spike `cc_version`: Ergebnis in `design.md`; die Stufe markiert im Diff, wenn die
  Fingerprint-Zeichen der ersten User-Nachricht verändert wurden.

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `redaction`: Redact-Stufe mit nummerierten Platzhaltern.
- `pipeline-core`: Stufenbericht (Diff, Treffer), Platzhalter-Zuordnung im Request,
  Sitzungs-Pinning über den Store.
- `bootstrap-config`: Schalter `redact.enabled`.
- `proxy`: Diff und Treffer der Stufen im gespeicherten Trace.

## Impact

Branch `issue-13/redact-stage`, gemeinsamer Workspace. Neue Dateien in `internal/redact/`,
`internal/pipeline/` (`pinner.go` und Tests), `internal/server/redact_stage_test.go`,
`cmd/tollgate/redact_wiring_test.go`; kleine eigene Hunks in
`internal/pipeline/pipeline.go`, `internal/pipeline/intake.go` (Kommentar), `internal/redact/redact.go`,
`internal/redact/doc.go`, `internal/config/config.go`,
`internal/server/server.go` (`recordTrace`) und `cmd/tollgate/main.go` (Stufenliste).
Keine Migration, kein neues Modul.

## Was ausdrücklich nicht

- Kein Rückweg, keine Rückübersetzung im Stream oder in Nicht-Streaming-Antworten (#14).
- Keine YAML-Konfiguration, Wortliste, Allowlist oder eigene Regex (#15).
- Kein Cache-Test mit Rückübersetzung über echte Turns (#17).
- Keine Änderung am `system`-Block wegen `cc_version` (bräuchte ein eigenes ADR).

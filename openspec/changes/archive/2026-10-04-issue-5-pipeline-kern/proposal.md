# Proposal

Issue: [#5 Pipeline-Kern: Stage-Interface, Trace-IDs, Eingang](https://github.com/niclasedge/tollgate/issues/5)

Geht zurück auf GitHub-Issue #5.

## Why

Alle Folge-Issues (Proxy #6, Client-Keys #8, Redact-Stage #13) hängen an einem gemeinsamen Pipeline-Kern: einem `Stage`-Interface, einer Trace-ID pro Anfrage, der Eingangsstufe und einer verlustfreien JSON-Bearbeitung. Ohne `jsonsplice` ließe sich die harte Regel „unbekannte Felder bleiben erhalten, Bytes ändern sich nicht" (ADR-0004) nicht erfüllen.

## What Changes

- `internal/jsonsplice`: strikter Scan eines `/v1/messages`-Bodys (Byte-Offsets der bearbeitbaren Strings, Blocktabelle aus ADR-0006), `Apply` für Ersetzungslisten und feste `Escape`-Funktion; Fuzz-Test.
- `internal/pipeline`: `Stage`-Interface mit Ersetzungslisten, `Request`-Kontext, `Runner` mit Trace pro Stufe (Dauer, geschätzte Tokens vorher/nachher, SHA-256 Ein-/Ausgabe, Konfigurationsversion, Ersetzungsliste), Trace-ID, Fehlerformat im Anthropic-Schema.
- Stufe 0 `Intake`: Client-Key über das Interface `KeyVerifier` prüfen, Body parsen, Sitzungsschlüssel ableiten, optional Konfigurationsversion über `ConfigPinner` ermitteln.
- Abhängigkeit `github.com/go-json-experiment/json` (Pseudo-Version) in `go.mod`.

## Capabilities

### New Capabilities

- `json-splicing`: Verlustfreie Bearbeitung ausgewählter JSON-Strings per Byte-Offsets.
- `pipeline-core`: Stage-Interface, Trace pro Anfrage und Stufe, Eingangsstufe.

### Modified Capabilities

Keine.

## Impact

- Neue Dateien in `internal/jsonsplice/` und `internal/pipeline/`; `go.mod`/`go.sum` erhalten nur `github.com/go-json-experiment/json`.
- Kein HTTP-Server, keine Key-Verwaltung, keine Datenbank, keine Redact- oder Compress-Logik.

## Was ausdrücklich nicht

- Kein HTTP-Server und keine Header-Weiterleitung (#6).
- Keine Schlüsselverwaltung, kein Hashing oder Speichern von Keys (#8); nur das Prüfer-Interface mit Test-Fake.
- Kein Sitzungs-Pinning in der Datenbank (#4/#13); nur das optionale Interface `ConfigPinner`.
- Keine Token-Schätzheuristik des Meters (#7); die Standardschätzung ist ein Platzhalter.
- Keine Redact-/Compress-Stufe.

## Berührte Dateien

- `go.mod`
- `go.sum`
- `internal/jsonsplice/doc.go`
- `internal/jsonsplice/scan.go`
- `internal/jsonsplice/splice.go`
- `internal/jsonsplice/jsonsplice_test.go`
- `internal/jsonsplice/fuzz_test.go`
- `internal/pipeline/doc.go`
- `internal/pipeline/pipeline.go`
- `internal/pipeline/intake.go`
- `internal/pipeline/errors.go`
- `internal/pipeline/pipeline_test.go`
- `internal/pipeline/intake_test.go`
- `openspec/changes/issue-5-pipeline-kern/**`

# Proposal

Issue: [#17 Phase-2-Abnahme: Cache-Test und E2E-Fixtures](https://github.com/niclasedge/tollgate/issues/17)

Depends on #16 (geschlossen). Verbindlich sind
[ADR-0006](../../../docs/adr/0006-platzhalter-sitzungen-rueckweg.md) (Cache-Test mit
Rückübersetzung und erneuter Redaktion, Sitzungs-Pinning) und
[ADR-0009](../../../docs/adr/0009-authentifizierung-und-geheimnisse.md) (Leck-Test über DB,
Logs und API).

## Why

Phase 2 (Redactor) ist mit #12 bis #16, #83 und #84 gebaut, aber nur stufen- und
paketweise getestet. Die Abnahme verlangt den Nachweis über das ganze Binary: mehrere Turns
einer Sitzung ergeben identische Präfixe, auch wenn Claude Code die rückübersetzten Antworten
im Klartext zurückschickt und mitten in der Sitzung eine neue Konfiguration aktiv wird; kein
Klartext-Geheimnis landet in DB, Logs oder Admin-API; die bereinigten Fixtures laufen Ende zu
Ende. Dazu kommt der Phasenbericht.

## What Changes

- **Abnahmetests** (`cmd/tollgate/acceptance_test.go`, neu): Sie starten `serve` mit
  eingeschalteter Redact-Stufe, echtem SQLite-Store, versionierter Konfiguration und
  Debug-Logs; nur CLIProxyAPI ist durch ein Mock-Modell (`httptest`) ersetzt, das die
  redigierte letzte Nutzernachricht als SSE zitiert (Thinking mit Platzhaltern und Signatur,
  Text, `tool_use` mit `input_json_delta`, Deltas zu 7 Bytes, jedes Event in zwei Writes).
  - `TestCacheStablePrefixAcrossTurns`: vier Turns einer Sitzung, Aktivierung einer neuen
    Version nach Turn 2, danach eine neue Sitzung.
  - `TestEndToEndFixtures`: jede Fixture aus `testdata/fixtures/`.
  - `TestNoPlaintextInDBLogsAndAdminAPI`: Leck-Suche nach Cache-Sitzung und Fixtures.
- **Fixture** `testdata/fixtures/multi-turn-session.json` (neu, synthetisch, nur erfundene
  Werte): drei Turns mit rückübersetzten Assistant-Blöcken, Thinking mit Signatur,
  `tool_result` als String und als Block-Array. `testdata/fixtures/README.md` beschreibt sie.
- **Phasenbericht** `docs/phasen/phase-2.md` (neu) mit Anleitung für die manuelle Abnahme
  „Claude Code normal über tollgate, Cache-Trefferquote nicht schlechter“.

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `redaction`: Abnahme-Anforderungen für Cache-Stabilität über das ganze Binary, Leck-Freiheit
  und End-to-End-Lauf der Fixtures.

## Impact

Branch `issue-17/phase2-abnahme`, gemeinsamer Workspace. Berührte Dateien:

- `cmd/tollgate/acceptance_test.go` (neu)
- `testdata/fixtures/multi-turn-session.json` (neu; wird über `tollgate.Fixtures()` auch
  im Dry-Run der Konfiguration verwendet)
- `testdata/fixtures/README.md`
- `docs/phasen/phase-2.md` (neu)
- `openspec/changes/issue-17-phase2-abnahme/` (dieser Change)

Keine Produktivdatei geändert: Alle Abnahmetests liefen gegen den vorhandenen Code grün,
sobald die Fixture existierte.

## Was ausdrücklich nicht

- Kein Lauf gegen echte Accounts oder Tokens. Die Abnahme „Claude Code normal über tollgate,
  Cache-Trefferquote nicht schlechter“ ist im Phasenbericht als manueller, offener Schritt
  mit Anleitung beschrieben.
- Kein Fix für den gefundenen Fall „bare Platzhalter eines Kontext-Treffers“ (siehe
  `design.md`, Abschnitt Befund). Er braucht eine Designentscheidung und ist als #97
  erfasst.

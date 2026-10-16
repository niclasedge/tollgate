# Proposal

Issue: [#20 Headroom-Port: Kompressionsstrategien](https://github.com/niclasedge/tollgate/issues/20)

> **Zusammen mit #19** auf einem Branch: `internal/compress` registriert die Strategien, die
> Strategiepakete hängen nur am Vertrag `compress/strat`. Die Capability `compression` ist noch
> nicht archiviert; dieser Change ergänzt sie als neue Requirements.

## Why

Die Compress-Stufe aus #19 braucht Strategien. Headroom hat deterministische Kompressoren
für Logs, Diffs und Suchergebnisse, die sich ohne ML und ohne Zustand nach Go portieren
lassen (ADR-0007).

## What Changes

- `internal/compress/logs`: `test_log` (Strategien 1 und 2) und `build_log` (5 und 2).
- `internal/compress/diff`: `diff` (Strategien 3 und 4).
- `internal/compress/search`: `search` (Strategie 6), weil `grep`/`rg`/`git grep` laut #18
  17,3 % der Bash-Bytes ausmachen; eingebaute Regel ohne `find`.
- Alle mit festen Obergrenzen statt `adaptive_sizer`, Golden-Files pro Strategie und
  Determinismustest.
- `third_party/headroom/LICENSE` und `NOTICE` unverändert, Eintrag in `third_party/NOTICE.md`
  mit Commit, portierten Funktionen und übernommenen Fixtures; Dateiköpfe mit Herkunft.

## Capabilities

### Modified Capabilities

- `compression`: Strategien `test_log`, `build_log`, `diff`, `search` und Herkunft.

## Impact

- Go: neue Pakete `internal/compress/{logs,diff,search}`, Registrierung in
  `internal/compress/compress.go`, Regel `search` in `config.go`.
- Doku: `third_party/NOTICE.md`, `docs/ARCHITECTURE.md` (Abschnitt Kompression).

## Was ausdrücklich nicht

- Kein Kompress/ModernBERT, Magika, CodeCompressor, SmartCrusher, CCR, ReadLifecycle,
  Prefix-Tracker (ADR-0007, „Nicht portiert“).
- Keine Regeln für `git log`/`git status` (1,3 %) und `ls`: eigenes Issue bei Bedarf.

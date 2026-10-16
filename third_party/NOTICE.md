# Herkunft und Lizenzen

## CLIProxyAPI

- **Quelle:** https://github.com/router-for-me/CLIProxyAPI
- **Commit:** `8ef43e4df3b216a42493105d31c2873b69191473`
- **Lizenz:** MIT; der Volltext liegt im Image unter
  `/usr/share/doc/cli-proxy-api/LICENSE`
- **Copyright:**
  - Copyright (c) 2025-2005.9 Luis Pater
  - Copyright (c) 2025.9-present Router-For.ME
- **Ort und Nutzung:** nicht im Repository. `deploy/cliproxyapi/Dockerfile` baut CLIProxyAPI
  per `go install` aus dem gepinnten Upstream-Modul; es läuft als eigener Prozess und wird
  nicht in tollgate importiert (ADR-0001, ADR-0014). Pin und Update-Ablauf:
  `deploy/cliproxyapi/UPSTREAM.md`.
- **Änderungen:** begrenzte Refresh-Barriere nur im temporären Image-Build (ADR-0016); Begründung in `cpa-refresh-barrier/PATCHES.md`. Katalog-Overlay für `claude-haiku-5-5`, ebenfalls nur im Build (ADR-0020); Begründung in `cpa-model-overlay/PATCHES.md`. Kein Upstream-Code im Repository.

Die Copyright-Zeilen sind unverändert aus der `LICENSE` des Upstream übernommen
(einschließlich der dortigen Schreibweise "2025-2005.9").

## Headroom

- **Quelle:** Headroom, Commit `0947a282eecc51bab0f52693303ac392e8111674` (2026-06-19),
  lokal als `ref/headroom` referenziert, nicht im Repository
- **Lizenz:** Apache-2.0, unverändert in `third_party/headroom/LICENSE`, dazu die
  unveränderte `third_party/headroom/NOTICE` (§4)
- **Copyright:** Copyright 2025 Headroom Contributors
- **Ort und Nutzung:** Go-Ports deterministischer Kompressoren aus
  `crates/headroom-core/` (ADR-0007). Übernommen sind Logik und Muster, kein Rust-Code.
  Jede portierte Datei nennt im Kopf Quelle, Stand und „portiert und verändert“.
  - `internal/compress/diff`: `src/transforms/diff_compressor.rs` (`parse_diff` mit der
    Erkennung von Datei- und Hunk-Köpfen, `reduce_context`, `select_hunks`, `score_hunks`
    samt Gewichten und `priority_patterns`, Dateigrenze aus `compress_with_store`) und
    `src/transforms/pipeline/offloads/diff_noise.rs` (`is_lockfile`,
    `body_is_whitespace_only`, `strip_ws`, `parse_new_path`). Die Lockfile-Liste stammt aus
    `config/pipeline.toml` (`[offload.diff_noise]`).
  - `internal/compress/search`: `src/transforms/search_compressor.rs` (Zeilenparser nach
    `parse_match_line`, Gruppierung nach Datei, Grenze pro Datei und Dateigrenze aus
    `select_matches`/`format_output`).
  - `internal/compress/logs`: `src/transforms/log_compressor.rs` (`LevelClassifier`,
    `StackTraceDetector`, `is_summary_line`, `score_log_line`, `parse_lines`,
    `select_lines`, `select_with_first_last`, `dedupe_similar`, `normalize_for_dedupe`) und
    `src/transforms/pipeline/reformats/log_template.rs` (`LogTemplate::apply`,
    `extends_run`, `merge_into_template`, `flush_run`).
- **Testdaten:** die Felder `input` der Fixtures `0cf5e189bd42fab1.json` und
  `5d950a94b8c22480.json` aus `tests/parity/fixtures/diff_compressor/` als
  `internal/compress/diff/testdata/diff/headroom_*.input`. Alle übrigen
  Golden-Eingaben sind synthetisch.
- **Änderungen:** ohne ML, Query-Scoring, `adaptive_sizer`, CCR und Footer; Köpfe und
  behaltene Zeilen bleiben byte-gleich; erweiterte Lockfile-Liste; Whitespace-Prüfung pro
  Hunk; Zeilennummer nur nach dem ersten Doppelpunkt; Normalisierer ersetzt Hex vor Dezimal;
  feste Obergrenzen; `build_log` kürzt lange Variantentabellen; begründet in
  `docs/adr/0007-kompression-und-headroom-port.md` und in den Dateiköpfen.

Die lokalen Referenzen unter `ref/` werden nicht versioniert.

## Go-Werkzeugabhängigkeiten

Sie sind in `go.mod` und `go.sum` gepinnt und werden über das Go-Modulsystem bezogen,
nicht im Repository kopiert.

## hook-redactor

- **Quelle:** hook-redactor v0.8.1 (eigenes Repo, lokal als `ref/hook-redactor` referenziert,
  nicht im Repository)
- **Lizenz:** MIT
- **Copyright:** Copyright (c) 2026 Niclas Edge
- **Ort und Nutzung:** `internal/redact` ist ein Go-Port des Pattern-Modus (Regex-Texte aus
  `src/patterns.rs`, False-Positive-Filter aus `src/redactor.rs`) und der Testfälle als
  Korpus unter `internal/redact/testdata/corpus/` (ADR-0005). Übernommen sind Regex-Texte, Filterlogik und Testfälle, kein Rust-Code.
- **Änderungen:** Auflösung aller Treffer in einem Durchgang statt sequenzieller Ersetzung,
  Secret-Gruppe statt Template, neues Muster für Verbindungs-URLs, nicht portierte Regeln
  und Modi; begründet in `docs/adr/0005-redactor-port.md`.

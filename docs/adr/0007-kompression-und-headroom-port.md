# ADR-0007: Kompression und Headroom-Port

- **Status:** Angenommen (Freigabe Niclas, 2026-10-04, PR #26)
- **Datum:** 2026-10-04
- **Issue:** #1

## Kontext

Die Compress-Stufe ersetzt rtk vollständig. Vorgaben aus dem Auftrag:
- Sie arbeitet nur auf `tool_result` von Bash-Aufrufen, mit einer Regel pro Befehl. Den
  Befehl liest sie aus dem zugehörigen `tool_use`.
- Read-, Edit- und andere Datei-Inhalte werden nie komprimiert.
- Alles ist deterministisch.

**Headroom** (`ref/headroom`, Apache-2.0, „Copyright 2025 Headroom Contributors“, mit
NOTICE) ist ein Kompressions-Proxy mit Python- und Rust-Teil. Befunde:

- **Command-Rewriting macht Headroom nicht selbst.** Es lädt das externe rtk-Binary
  (`headroom/rtk/__init__.py`). rtk im Proxy auszuführen wurde dort „considered and rejected“,
  wegen des Cache-Risikos (`docs/rtk-architecture.md`).
- **Routing nur nach Inhaltstyp und Tool-Name.** Eine Erkennung pro Befehl gibt es nicht.
  `DEFAULT_EXCLUDE_TOOLS = {Read, Glob, Grep, Write, Edit}` und `Bash: moderate`
  (`headroom/config.py`).
- **Cache-Strategie: nur der letzte Turn.**
  - Die Rust-Live-Zone komprimiert nur die letzte User-Nachricht oberhalb der höchsten
    `cache_control`-Markierung (`transforms/live_zone.rs`, `cache_control.rs`).
  - Headroom räumt selbst ein, dass das den Cache im nächsten Turn bricht
    (`headroom/proxy/handlers/anthropic.py`: „The latest user turn becomes historical on the
    next request …“).
  - Ausgleich über Sitzungszustand (`cache/prefix_tracker.py`, `cache/compression_cache.py`).
- **Brauchbare Invarianten** (`REALIGNMENT/02-architecture.md`):
  - bytegenaues Durchreichen
  - Hot-Zone-Blöcke (`thinking`, `redacted_thinking`, `tool_use`, `compaction`) nie
    verändert
  - Determinismus (BTreeMap, keine Zeit)
  - Mindestgröße 512 Bytes, Ersetzung nur bei echter Ersparnis
- **Deterministische Kompressoren** in `crates/headroom-core/src/transforms/`:
  - `log_compressor.rs`
  - `diff_compressor.rs`
  - `pipeline/offloads/diff_noise.rs`
  - `pipeline/reformats/log_template.rs`
  - `search_compressor.rs`
- **ML-basiert oder zustandsbehaftet:** Kompress (ModernBERT), Magika, CodeCompressor
  (tree-sitter), SmartCrusher (BM25 + Embeddings), CCR-Store mit `headroom_retrieve`-Tool,
  ReadLifecycle, Learn/TOIN.

## Entscheidung

### Grundsätze der Compress-Stufe

1. **Zustandslos über den ganzen Verlauf.** In jedem Turn läuft dieselbe reine Funktion
   `compress(befehl, ausgabe, regelkonfig) → ausgabe'` über **jedes** `tool_result` im Request,
   dessen `tool_use_id` auf einen `tool_use` mit `name == "Bash"` zeigt. Gleiche Eingabe ergibt
   gleiche Ausgabe. Der Präfix bleibt dadurch über alle Turns stabil, ohne Prefix-Tracker und
   ohne Sitzungszustand. `regelkonfig` kommt aus der gepinnten Version (ADR-0006, ADR-0008).
2. **Nur Bash.** `Read`, `Edit`, `Write`, `Grep`, `Glob`, MCP-Tools und alle unbekannten Tools
   werden nie angefasst. Ein Test prüft das für jede dieser Tool-Arten.
3. **Regelauswahl über `tool_use.input.command`.**
   - Der Befehl wird normalisiert: führende Env-Zuweisungen und `cd …&&` entfernt, Whitespace
     zusammengefasst.
   - Danach gegen die `match`-Präfixe der Regeln geprüft, in Konfigurationsreihenfolge. Die
     erste passende Regel gewinnt.
   - Pipes, Subshells und Befehlsketten: Die genaue Auflösung legt #19 fest. Bis dahin gilt:
     Passt kein Präfix eindeutig, wird nicht komprimiert.
4. **Gates.** Ersetzt wird nur, wenn die Ausgabe ≥ `min_bytes` ist (Standard 2048) und das
   Ergebnis strikt kleiner ist. Sonst bleibt das Original unverändert. Ein Byte-Gate reicht,
   einen Tokenizer braucht es nicht.
5. **Struktur bleibt erhalten.**
   - `tool_use_id`, `is_error` und die Blockreihenfolge bleiben unverändert.
   - Ist `tool_result.content` ein Array, werden nur `text`-Blöcke komprimiert, alle anderen
     (z. B. `image`) bleiben unverändert.
   - Ersetzt wird per Byte-Splicing (ADR-0004).
6. **Reihenfolge der Stufen: Redact vor Compress.** Compress sieht also nur redigierten Text.
   Entfernt Compress Zeilen, verschwinden darin enthaltene Platzhalter. Das ist unkritisch,
   weil die Zuordnung pro Anfrage gilt (ADR-0006).
7. **Kennzeichnung.** Jede komprimierte Ausgabe endet mit einer festen Zeile im Format
   `[tollgate: <regel> · <n> Zeilen ausgelassen · <details>]`. Das Modell weiß dann, dass
   gekürzt wurde. Die Zeile hängt nur vom Ergebnis ab, nicht von Zeit oder Version.

### Aus Headroom portiert

| # | Strategie | Quelle (`crates/headroom-core/src/transforms/`) | Einsatz in tollgate | Anpassung |
|---|---|---|---|---|
| 1 | LogCompressor: Level-Klassifikator, Stacktrace-Zustandsautomat (Python, JS, Java, Rust, Go), Summary-Erkennung, Auswahl erster/letzter Fehler plus Top-N mit ±N Kontextzeilen, Footer | `log_compressor.rs` | Strategie `test_log` für `cargo test`, `go test`, `pytest`, `npm test`/`jest`, `make` | feste Obergrenzen pro Regel statt `adaptive_sizer`; Format-Erkennung durch den Befehl statt durch Inhaltsheuristik |
| 2 | Dedupe-Normalisierer für Warnungen | `log_compressor.rs` (`normalize_for_dedupe`) | Teil von `test_log` und `build_log` | Hex vor Dezimal ersetzen (in Headroom vertauscht) |
| 3 | DiffCompressor: Kontext kürzen, Obergrenzen für Dateien und Hunks, Summary | `diff_compressor.rs` | Strategie `diff` für `git diff`, `git show`, `git log -p` | ohne Query-Scoring |
| 4 | DiffNoise: Lockfile- und Whitespace-Hunks entfernen | `pipeline/offloads/diff_noise.rs` | Teil von `diff` | ohne CCR: Die Summary nennt die entfernten Dateien |
| 5 | LogTemplate: Läufe ≥ 3 gleichartiger Zeilen zusammenfassen | `pipeline/reformats/log_template.rs` | Strategie `build_log` für `npm install`, `pnpm install`, `docker build`, `cargo build` | – |
| 6 | SearchCompressor (optional, nach Daten aus #18) | `search_compressor.rs` | Strategie `search` für `grep`/`rg` in Bash | nur wenn die Token-Analyse es rechtfertigt |

Nicht aus Headroom, aber in #19 nach den Daten aus #18 vorgesehen: einfache Regeln für
`git log` (Einzeiler-Format) und `git status` (Gruppierung). Welche Befehle zuerst kommen,
entscheidet die Token-Analyse aus Phase 1 (#18).

### Nicht portiert

| Teil | Grund |
|---|---|
| Kompress, Magika, UniversalCompressor, CodeCompressor, SmartCrusher/HybridScorer, Embeddings | ML oder tree-sitter, nicht deterministisch bzw. nicht cgo-frei |
| `adaptive_sizer` | Die zlib-Längenprüfung ist nicht byte-portabel zwischen Implementierungen. Feste Obergrenzen sind nachvollziehbarer |
| CCR (`<<ccr:…>>`, `headroom_retrieve`-Tool, Compression Store) | Zustand, verändert `tools`, braucht einen Tool-Roundtrip |
| ReadLifecycle | komprimiert Read-Ausgaben, verletzt eine harte Regel |
| Prefix-Tracker, Compression-Cache, „nur letzter Turn“ | unnötig durch den zustandslosen Ansatz über den ganzen Verlauf |
| Learn/TOIN, Recommendations | Zustand, außerhalb des Umfangs |
| Output-Shaper, automatisches `cache_control`, Tool-Sortierung | verändern `system`, `tools` oder `effort` |
| tiktoken-Token-Gate | Ein Byte-Gate reicht und ist deterministisch |

### Lizenz und Herkunft

- Apache-2.0 §4: `third_party/headroom/LICENSE` und `third_party/headroom/NOTICE`
  (unverändert) werden übernommen, Eintrag in `third_party/NOTICE.md`.
- Jede portierte Go-Datei trägt einen Kopf mit Quelldatei, Headroom-Stand und dem Hinweis
  „portiert und verändert“.

### Tests

- Golden-Files pro Regel unter `internal/compress/testdata/<regel>/` mit `*.input`,
  `*.golden` und `*.cmd` (Befehl), aktualisierbar über `go test -update`.
- Headroom-Fixtures `tests/parity/fixtures/diff_compressor/*.json` (27 Fälle) sind als
  Eingaben für `diff` brauchbar, ohne die Felder `recorded_at` und CCR-`hash=`.
- Die Log-Fixtures sind fast alle unter 50 Zeilen und bleiben in Headroom unverändert. Für
  `test_log` entstehen deshalb eigene Golden-Files aus echten, bereinigten Ausgaben.
- Determinismus- und Byte-Treue-Tests laut ADR-0004.

## Alternativen

- **rtk als Subprozess im Proxy:** verworfen. Externe Abhängigkeit, Versionsdrift, und
  Headroom hat es aus Cache-Gründen ebenfalls verworfen.
- **Nur den letzten Turn komprimieren** wie Headroom: verworfen. Das bricht den Cache im
  Folgeturn oder braucht Sitzungszustand.
- **Inhaltsbasierte Erkennung statt Befehl:** verworfen als Hauptmechanismus. Der Auftrag
  verlangt Regeln pro Befehl, und der Befehl ist eindeutiger als eine Heuristik.

## Konsequenzen

- **#18** liefert die Rangliste der Befehle.
- **#19** setzt die Stufe, das Matching und die ersten Regeln um.
- **#20** portiert die Strategien 1–5 (6 optional).
- **#21** zeigt im Playground jede Strategie Schritt für Schritt.

## Quellen

Headroom (lokal, Stand 2026-10-04):
- `crates/headroom-core/src/transforms/log_compressor.rs`, `diff_compressor.rs`,
  `search_compressor.rs`, `live_zone.rs`
- `crates/headroom-core/src/transforms/pipeline/offloads/diff_noise.rs`,
  `pipeline/reformats/log_template.rs`
- `crates/headroom-core/tests/live_zone_dispatch.rs`
- `headroom/config.py`, `headroom/proxy/handlers/anthropic.py`, `docs/rtk-architecture.md`
- `REALIGNMENT/02-architecture.md`, `tests/parity/fixtures/`, `LICENSE`, `NOTICE`

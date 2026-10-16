## ADDED Requirements

### Requirement: Compress-Stufe nur für Bash-Ausgaben

Die Stufe `compress` SHALL nach `redact` laufen und nur `/v1/messages` und `/v1/messages/count_tokens` bearbeiten. Sie SHALL in jedem Request jedes `tool_result` (und dessen `text`-Blöcke) bearbeiten, dessen `tool_use_id` auf einen `tool_use` mit `name == "Bash"` zeigt, mit dem Befehl aus dessen `input.command`. Ergebnisse von Read, Edit, Write, Grep, Glob, MCP- und unbekannten Tools MUST byte-gleich bleiben. `tool_use_id`, `is_error`, Nicht-Text-Blöcke und die Blockreihenfolge MUST unverändert bleiben. Die Ausgabe MUST nur von Befehl, Ausgabe und Regelkonfiguration abhängen.

#### Scenario: Read-Ergebnis
- **WHEN** ein Request ein großes Read-Ergebnis und ein großes Bash-Ergebnis von `go test` enthält
- **THEN** wird nur das Bash-Ergebnis gekürzt, das Read-Ergebnis bleibt byte-gleich.

#### Scenario: Ganzer Verlauf
- **WHEN** derselbe Verlauf mit einer weiteren Nachricht erneut gesendet wird
- **THEN** sind alle früheren Bash-Ergebnisse byte-gleich gekürzt wie im ersten Request.

### Requirement: Befehlserkennung

Vor der Regelauswahl SHALL der Befehl normalisiert werden: Segmente `cd`, `pushd`, `export` und `set` einer Kette, führende Env-Zuweisungen, `time` und `env`, Umleitungen nach `/dev/null` oder `2>&1`, `git --no-pager` sowie Verzeichnis-Optionen (`git -C`/`-c`, `make -C`, `pnpm -C`/`--dir`) entfallen, Whitespace wird zusammengefasst. Enthält der Befehl eine Pipe, `||`, Befehlssubstitution, ein Heredoc, ein offenes Anführungszeichen oder bleibt mehr als ein Befehl übrig, MUST die Ausgabe unverändert bleiben. Text in Anführungszeichen MUST NOT geteilt werden.

#### Scenario: Kette mit cd
- **WHEN** der Befehl `cd web && CI=1 pnpm test 2>&1` lautet
- **THEN** wird `pnpm test` erkannt.

#### Scenario: Pipeline
- **WHEN** der Befehl `go test ./... | tail -50` lautet
- **THEN** bleibt die Ausgabe unverändert.

### Requirement: Regeln und Gates

Die Regeln SHALL in dieser Reihenfolge geprüft werden: zuerst die Regeln aus `compress.rules` der gepinnten Konfigurationsversion, dann die eingebauten; ein `match`-Präfix passt nur auf ganze Wörter, die erste passende Regel gewinnt. Die Strategie `passthrough` MUST die Ausgabe unverändert lassen. Ersetzt MUST nur werden, wenn die Ausgabe mindestens `min_bytes` (Standard 2048) lang ist und das Ergebnis samt Kennzeichnungszeile strikt kürzer ist. Jede gekürzte Ausgabe MUST mit der Zeile `[tollgate: <regel> · <n> Zeilen ausgelassen · <details>]` enden, die nur vom Ergebnis abhängt.

#### Scenario: Konfigurierte Regel schaltet ab
- **WHEN** die Version eine Regel `match: ["go test"]` mit `strategy: passthrough` enthält
- **THEN** bleibt die Ausgabe von `go test` unverändert, obwohl eine eingebaute Regel passt.

#### Scenario: Kleine Ausgabe
- **WHEN** die Ausgabe kürzer als `min_bytes` ist
- **THEN** bleibt sie unverändert.

### Requirement: Trace der Kompression

Die Stufe SHALL im Trace je gekürzter Ausgabe eine Zeile mit Pfad, Regel, Bytes vorher und nachher und Zahl der ausgelassenen Zeilen speichern, höchstens 500 Zeilen. Der Trace MUST NOT Ausgabetext enthalten.

#### Scenario: Bericht
- **WHEN** ein `go test`-Ergebnis von 9000 auf 1200 Bytes gekürzt wird
- **THEN** enthält der Trace `messages[2].content[0] · go_test · 9000 → 1200 Bytes · … Zeilen ausgelassen`.

### Requirement: Schalter für die Kompression

Die Stufe SHALL nur laufen, wenn `compress.enabled` bzw. `TOLLGATE_COMPRESS_ENABLED` gesetzt ist; im Binary MUST der Standard aus sein. `docker-compose.yml` MUST den Schalter mit Standard `true` an den tollgate-Container reichen, damit `just setup` immer mit Kompression startet; `TOLLGATE_COMPRESS_ENABLED=false` in `.env` schaltet sie ab.

#### Scenario: Standard im Binary
- **WHEN** der Schalter nicht gesetzt ist
- **THEN** enthält die Pipeline keine Stufe `compress`, und Bash-Ausgaben gehen unverändert weiter.

#### Scenario: Standard im Container
- **WHEN** `.env` keine Zeile `TOLLGATE_COMPRESS_ENABLED` enthält
- **THEN** startet der tollgate-Container mit `TOLLGATE_COMPRESS_ENABLED=true`.

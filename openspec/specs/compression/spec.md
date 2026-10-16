# compression Specification

## Purpose
TBD - created by archiving change issue-18-token-analyse. Update Purpose after archive.

## Requirements

### Requirement: Regelauswahl nach Token-Analyse

Die Auswahl der eingebauten Kompressionsregeln SHALL auf der Rangliste in `docs/token-analyse.md` beruhen. Die Rangliste MUST Methode, Datenquelle, Zeitraum, Bytes je Befehlsgruppe und den Anteil der Ausgaben ab `min_bytes` nennen. Sie MUST NOT Projektnamen, Pfade oder Inhalte der ausgewerteten Ausgaben enthalten.

#### Scenario: Dateilesen per Bash
- **WHEN** die Rangliste `sed -n`, `cat`, `head` und `tail` als größten Block ausweist
- **THEN** bekommen diese Befehle keine eingebaute Regel, weil ihre Ausgabe Dateiinhalt ist.

#### Scenario: Suche gerechtfertigt
- **WHEN** `grep`, `rg` und `git grep` zusammen über 10 % der Bash-Bytes ausmachen
- **THEN** wird die optionale Strategie `search` aus ADR-0007 umgesetzt.

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

### Requirement: Strategien test_log und build_log

Die Strategie `test_log` SHALL Ausgaben von Testläufen (`go test`, `cargo test`, `pytest`, `pnpm test`/`vitest`/`jest`, `make test`, `just test`) kürzen. Sie MUST alle Fehlschlag-Markierungen (`--- FAIL`, `FAIL <paket>`, `panic`, `FAILED <test>`, pytest-`E`-Zeilen, die cargo-Liste `failures:`) und alle Zusammenfassungen behalten. Dazu behält sie den ersten und letzten Fehler, die `max_errors` (Standard 5) wichtigsten weiteren Fehler mit ±`context_lines` (Standard 3), höchstens 5 deduplizierte Warnungen und die ersten 3 Stacktraces mit je höchstens 20 Zeilen. Zeilen bestandener Tests MUST NOT als Fehler zählen. Die Strategie `build_log` SHALL Installations- und Buildausgaben kürzen: Läufe von mindestens 3 gleichartigen Zeilen werden zu einer Vorlagenzeile mit Variantentabelle, wiederholte Warnblöcke erscheinen einmal. Fehler, Warnungen, Zusammenfassungen und die letzten 3 Zeilen MUST unverändert bleiben. Ausgaben unter 10 Zeilen MUST unverändert bleiben.

#### Scenario: go test mit zwei Fehlschlägen
- **WHEN** `go test ./...` zwei fehlschlagende Tests unter rund 50 Paketen mit `ok` ausgibt
- **THEN** bleiben alle `--- FAIL`-Zeilen samt Meldungen und `FAIL`-Zeilen stehen, und die Kennzeichnung nennt die Zahl der ok-Pakete.

#### Scenario: npm install
- **WHEN** `npm install` 30 gleichartige `npm http fetch`-Zeilen ausgibt
- **THEN** stehen eine Vorlagenzeile mit Varianten und die vollständige Abschlusszusammenfassung im Ergebnis.

### Requirement: Strategie diff

Die Strategie `diff` SHALL Ausgaben von `git diff`, `git show`, `git log -p` und `git stash show -p` kürzen: Hunks in Lockfiles und reine Whitespace-Hunks entfallen, je Ausgabe höchstens `max_files` Dateien (Standard 30) und je Datei höchstens `max_hunks` Hunks (Standard 20) bleiben stehen, unveränderte Zeilen über `context_lines` (Standard 3) hinaus entfallen. Commit-Köpfe, Datei-Köpfe, `@@`-Zeilen und alle behaltenen `+`/`-`-Zeilen MUST byte-gleich bleiben. An jeder Stelle mit ausgelassenen Hunks MUST eine Notizzeile mit Anzahl und Grund stehen; ausgelassene Dateien MUST in der Kennzeichnungszeile genannt werden. Eine Ausgabe ohne `diff --git`-Abschnitt MUST unverändert bleiben.

#### Scenario: Lockfile
- **WHEN** `git diff` eine Änderung in `pnpm-lock.yaml` und eine in `src/app.ts` zeigt
- **THEN** bleibt der Hunk von `src/app.ts` byte-gleich, der Lockfile-Hunk wird zu `[tollgate: 1 Hunk ausgelassen (Lockfile, … Zeilen)]`.

#### Scenario: Mehrere Commits
- **WHEN** `git log -p -3` drei Commits mit Diffs ausgibt
- **THEN** bleiben alle drei Commit-Köpfe vollständig erhalten.

### Requirement: Strategie search

Die Strategie `search` SHALL Ausgaben von `grep`, `egrep`, `fgrep`, `rg` und `git grep` kürzen. Erkannt werden die Formen `pfad:zeile:text` mit Kontextzeilen, `rg --heading` und Pfadlisten ohne Whitespace. Je Datei MUST höchstens `max_errors` Treffer (Standard 8) und insgesamt höchstens `max_files` Dateien (Standard 40) in der Reihenfolge des ersten Auftretens stehen bleiben, jeweils mit einer Zusammenfassungszeile, die mit `… ` beginnt. Behaltene Zeilen MUST byte-gleich bleiben, außer Trefferzeilen über 300 Bytes, die auf 200 Bytes an einer UTF-8-Grenze plus `…` gekürzt werden. Lässt sich weniger als die Hälfte der Zeilen als Suchergebnis lesen, MUST die Ausgabe unverändert bleiben. `find` MUST NOT über `search` laufen.

#### Scenario: Viele Treffer in einer Datei
- **WHEN** `grep -rn TODO src` in einer Datei 22 Treffer liefert
- **THEN** bleiben die ersten 8 stehen, gefolgt von `… <datei>: 14 weitere Treffer`.

#### Scenario: Treffer ohne Pfad
- **WHEN** `grep step file.go` nur Textzeilen ohne Pfadpräfix liefert, auch wenn sie `/` enthalten
- **THEN** bleibt die Ausgabe unverändert.

### Requirement: Herkunft der Ports

Jede aus Headroom portierte Go-Datei SHALL im Kopf Quelldatei, Headroom-Stand und „portiert und verändert“ nennen. `third_party/headroom/LICENSE` und `third_party/headroom/NOTICE` MUST unverändert vorliegen; `third_party/NOTICE.md` MUST Quelle, Commit, Lizenz, portierte Funktionen, übernommene Fixtures und Änderungen nennen. Jede Strategie MUST Golden-Files unter `testdata/<strategie>/` haben, deren zweiter Lauf byte-gleich ist.

#### Scenario: Golden-Test
- **WHEN** `go test ./internal/compress/...` läuft
- **THEN** stimmt jede Ausgabe mit ihrer `.golden`-Datei überein und ist bei zweimaligem Lauf identisch.

### Requirement: Erklärung der Kompression

tollgate SHALL für einen Befehl und seine Ausgabe erklären, was die Compress-Stufe tut: den normalisierten Befehl, die passende Regel und Strategie, den Text nach jedem Schritt der Strategie und das Ergebnis mit Markerzeile. Ergebnis und Entscheidung MUST mit der Compress-Stufe übereinstimmen. Bleibt die Ausgabe unverändert, MUST der Grund genannt werden. Die Schritte SHALL auch dann gezeigt werden, wenn nur das Größen-Gate greift. Text aus dem Playground MUST NOT gespeichert oder geloggt werden.

#### Scenario: Ausgabe wird komprimiert
- **WHEN** `go test ./...` mit einer langen fehlschlagenden Ausgabe geprüft wird
- **THEN** zeigt die Erklärung Eingabe, die Schritte von `test_log` und das Ergebnis, das Byte für Byte der Ausgabe der Compress-Stufe entspricht.

#### Scenario: Pipe
- **WHEN** `go test ./... | tail` geprüft wird
- **THEN** bleibt die Ausgabe unverändert, und der Grund nennt, dass es kein einzelner Befehl ist.

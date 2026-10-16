## ADDED Requirements

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

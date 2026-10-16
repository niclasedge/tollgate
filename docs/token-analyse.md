# Token-Analyse: teuerste Bash-Befehle

Grundlage für die Priorisierung der Kompressionsregeln (Issue #18).

## Methode

- **Datenquelle:** Die tollgate-DB speichert keine Request-Bodies. Als Ersatz für die
  Phase-1-Daten dienen die lokalen Claude-Code-Transkripte (JSONL) eines Entwicklers,
  inklusive Subagenten-Transkripte. Die Dateien wurden nur gelesen.
- **Zeitraum:** 15.08.2026 bis 05.10.2026.
- **Umfang:** 794 Sitzungen (513 davon mit Bash-Aufrufen) aus 46 Projektverzeichnissen,
  32.367 Bash-Aufrufe mit 54,6 MB Ausgabe (≈ 13,7 Mio. Tokens).
- **Join:** `tool_use` (`name == "Bash"`, `input.command`, `id`) wird über
  `tool_use_id` mit dem `tool_result` verbunden; gemessen wird die UTF-8-Länge des
  Ergebnistexts (String oder Summe der `text`-Blöcke). Doppelte IDs zählen einmal.
- **Tokens:** grob geschätzt als Bytes / 4.
- **Gruppierung:** Vom Befehl werden führende Umgebungsvariablen (`FOO=bar`), `cd … &&`,
  `export …;`, `set -e;`, `timeout N`, `2>&1` und ein führendes `rtk ` bzw. `rtk proxy `
  entfernt. Gruppe = erstes Wort, bei Mehrbefehls-Werkzeugen plus Unterbefehl
  (`git diff`, `go test`, `gh issue view`, `uv run python` …), `make <target>` → `make`.
  Bei Ketten und Pipelines zählt der **erste** Befehl. Alles, was kein allgemein bekanntes
  Werkzeug ist (projekteigene Binaries, Skripte), steht in `andere`.

## Ranking (Top 25, alle Bash-Aufrufe)

| # | Gruppe | Aufrufe | Bytes gesamt | Median | p90 | ≈ Tokens | Anteil | davon ≥ 2048 B |
|---:|---|---:|---:|---:|---:|---:|---:|---:|
| 1 | `sed` | 3.366 | 11.081.956 | 2.585 | 6.711 | 2.770.489 | 20,3 % | 88 % |
| 2 | `cat` | 2.273 | 9.763.143 | 1.764 | 13.150 | 2.440.785 | 17,9 % | 93 % |
| 3 | `grep` | 5.888 | 8.712.683 | 919 | 3.477 | 2.178.170 | 15,9 % | 62 % |
| 4 | `andere` | 2.902 | 3.605.173 | 410 | 2.572 | 901.293 | 6,6 % | 66 % |
| 5 | `ls` | 1.929 | 3.053.832 | 745 | 4.358 | 763.458 | 5,6 % | 70 % |
| 6 | `echo` | 1.610 | 2.302.837 | 877 | 3.172 | 575.709 | 4,2 % | 59 % |
| 7 | `for`-Schleife | 761 | 2.103.697 | 784 | 7.818 | 525.924 | 3,9 % | 85 % |
| 8 | `python` | 1.086 | 1.381.212 | 558 | 3.172 | 345.303 | 2,5 % | 62 % |
| 9 | `uv run python` | 1.125 | 1.358.314 | 406 | 3.018 | 339.578 | 2,5 % | 68 % |
| 10 | `git diff` | 244 | 690.130 | 1.555 | 6.909 | 172.532 | 1,3 % | 86 % |
| 11 | `node` | 738 | 685.124 | 463 | 2.222 | 171.281 | 1,3 % | 47 % |
| 12 | `bd show` | 196 | 647.039 | 2.343 | 7.020 | 161.759 | 1,2 % | 87 % |
| 13 | `find` | 295 | 434.675 | 727 | 3.627 | 108.668 | 0,8 % | 66 % |
| 14 | `gh issue view` | 155 | 413.688 | 1.433 | 5.990 | 103.422 | 0,8 % | 83 % |
| 15 | `head` | 225 | 413.497 | 1.371 | 3.415 | 103.374 | 0,8 % | 63 % |
| 16 | `git status` | 462 | 404.450 | 464 | 1.748 | 101.112 | 0,7 % | 45 % |
| 17 | `tail` | 178 | 314.152 | 912 | 3.538 | 78.538 | 0,6 % | 70 % |
| 18 | `git log` | 230 | 306.459 | 662 | 2.720 | 76.614 | 0,6 % | 57 % |
| 19 | `git show` | 113 | 298.727 | 1.096 | 8.039 | 74.681 | 0,5 % | 84 % |
| 20 | `wc` | 187 | 280.159 | 582 | 3.298 | 70.039 | 0,5 % | 67 % |
| 21 | `just test` | 225 | 250.433 | 669 | 2.512 | 62.608 | 0,5 % | 44 % |
| 22 | `just` | 216 | 248.669 | 633 | 2.725 | 62.167 | 0,5 % | 56 % |
| 23 | `sleep` | 397 | 222.414 | 225 | 1.442 | 55.603 | 0,4 % | 38 % |
| 24 | `git grep` | 59 | 221.530 | 1.647 | 9.807 | 55.382 | 0,4 % | 91 % |
| 25 | `uv run` | 235 | 178.129 | 398 | 1.809 | 44.532 | 0,3 % | 36 % |
| – | übrige Gruppen | 7.272 | 5.254.512 | – | – | 1.313.628 | 9,6 % | – |

Median und p90 in Bytes. „davon ≥ 2048 B“ = Anteil der Bytes der Gruppe, der in
Ausgaben ab 2048 Bytes steckt.

Typische Test- und Build-Befehle liegen weit hinten: `go test` 291 Aufrufe / 113 KB
(0,2 %), `pnpm exec` 118 / 138 KB, `uv run pytest` 77 / 67 KB, `make` 57 / 41 KB,
`go build` 148 / 30 KB, `rg` 58 / 70 KB, `cargo test` und `npm install` praktisch nicht
vorhanden.

### Ranking ohne rtk-Aufrufe

Ohne die 1.018 rtk-Aufrufe ändert sich die Reihenfolge der Top 10 nicht (einzige
Verschiebung: `node` vor `git diff`). Die Spitze bleibt `sed` 20,2 %, `cat` 18,3 %,
`grep` 15,4 %, `andere` 6,6 %, `ls` 5,7 %, `echo` 4,3 %, `for`-Schleife 4,0 %.

## Mindestgröße 2048 Bytes (`min_bytes`)

- 7.628 von 32.367 Ausgaben (23,6 %) sind ≥ 2048 Bytes.
- Sie halten 40,1 MB, also **73,4 %** aller Bash-Ausgabe-Bytes.
- Die Schwelle lässt drei Viertel der Aufrufe unberührt und erreicht trotzdem fast drei
  Viertel des Volumens.
- Die größten Einzelausgaben liegen knapp unter 30.000 Bytes; Claude Code kappt
  Bash-Ausgaben dort bereits selbst.

## rtk-Anteil

- 1.018 Aufrufe (3,1 %, 2,6 % der Bytes) beginnen mit `rtk`.
- Davon sind 993 `rtk proxy`, also **ungefiltert** durchgereicht. Nur 25 Aufrufe
  (0,1 % der Bytes) wurden tatsächlich von rtk komprimiert.
- Die Daten sind damit praktisch Rohausgaben und taugen direkt als Basis für die Regeln.

## Pipelines und Ketten

- **Pipelines (`|`):** 23.006 Aufrufe (71 %), 33,0 MB (60 % der Bytes). Häufigste
  Anfänge: `grep`, `cat`, `ls`, `sed`, `echo`, `uv run python`, `for`-Schleife.
- **Ketten (`&&`, `;`, `||`):** 20.993 Aufrufe (65 %), 35,5 MB (65 % der Bytes).
  Häufigste Anfänge: `grep`, `sed`, `cat`, `ls`, `echo`, `for`-Schleife.
- 15.141 Pipelines enden mit `| head` oder `| tail`, 745 mit `| grep`/`| rg`. Das Modell
  kürzt also bereits selbst.
- Testläufe: 2.411 Aufrufe enthalten irgendwo einen Testbefehl (`go test`, `pytest`,
  `vitest`, `just test`, `make test` …), zusammen nur 1,6 MB (3,0 %). 89 % davon sind
  schon mit `| head`, `| tail` oder `| grep` gekürzt. Buildläufe: 997 Aufrufe, 0,5 MB (1,0 %).
- Folge: Eine Regel, die nur auf das erste Wort schaut, trifft bei Ketten oft den
  falschen Befehl. Die Befehlserkennung muss Ketten aufteilen und den Befehl wählen, der
  die Ausgabe dominiert; bei bereits mit `head`/`tail` gekürzten Pipelines lohnt sich
  Kompression kaum.

## Kontext: Bytes pro Werkzeug

| Werkzeug | Aufrufe | Bytes | Anteil | Ø Bytes/Aufruf |
|---|---:|---:|---:|---:|
| Bash | 32.367 | 54.626.634 | 65,1 % | 1.687 |
| Read | 4.566 | 26.414.749 | 31,5 % | 5.785 |
| Edit | 5.102 | 924.510 | 1,1 % | 181 |
| WebFetch | 47 | 455.497 | 0,5 % | 9.691 |
| Agent | 393 | 389.539 | 0,5 % | 991 |
| Write | 2.104 | 371.202 | 0,4 % | 176 |
| WebSearch | 28 | 129.035 | 0,2 % | 4.608 |
| übrige (u. a. MCP-Werkzeuge) | – | 616.073 | 0,7 % | – |

Bash liefert doppelt so viele Bytes wie Read. Ein großer Teil davon sind aber selbst
Dateiinhalte (siehe unten).

## Empfehlung

1. **Dateilesen per Bash nicht komprimieren.** `sed -n` (2.953 Aufrufe, 19,6 %), `cat`
   auf Dateien (1.535 Aufrufe, 17,1 %), `head` und `tail` machen zusammen 39,5 % der
   Bash-Bytes aus. Das sind Dateiinhalte; die harte Regel „niemals Datei-Inhalte
   komprimieren“ gilt sinngemäß auch hier. Die Befehlserkennung soll diese Fälle
   ausdrücklich als Dateilesen einstufen und unverändert lassen.
2. **`search` zuerst (`grep`, `rg`, `git grep`, `find`).** 6.300 Aufrufe, 9,4 MB =
   17,3 % der Bash-Bytes; davon 5,9 MB (10,8 %) in Ausgaben ≥ 2048 Bytes. Das ist mit
   Abstand der größte komprimierbare Block. Die optionale `search`-Strategie ist damit
   **klar gerechtfertigt**: Treffer pro Datei gruppieren, Pfadpräfixe zusammenfassen,
   lange Zeilen kürzen, Treffer pro Datei begrenzen. `ls` (5,6 %) passt als
   Verzeichnisliste in dieselbe Familie.
3. **`diff` als Zweites (`git diff`, `git show`).** 357 Aufrufe, 1,0 MB (1,8 %), aber
   86 % der Bytes in großen Ausgaben und hohe p90-Werte (7–8 KB). Kontextzeilen
   kürzen, Lockfiles und generierte Dateien zusammenfassen.
4. **Neue einfache Regel für `git log` und `git status`.** Zusammen 692 Aufrufe,
   0,7 MB (1,3 %). Kleiner Hebel, aber triviale und sichere Regel (einzeilige Commits,
   Status kompakt).
5. **`test_log` und `build_log` mit geringerer Priorität.** In diesen Daten nur 3,0 %
   (Test) bzw. 1,0 % (Build) der Bytes, und fast immer schon vom Modell gekürzt. Die
   Regeln bleiben sinnvoll, weil Testausgaben bei Fehlern schnell wachsen, bringen hier
   aber wenig messbare Ersparnis.
6. **`min_bytes = 2048` beibehalten.** Die Schwelle trifft 73 % des Volumens bei nur
   24 % der Aufrufe.
7. **Offen:** `echo`, `for`-Schleifen, `python`/`uv run python` und `andere` (zusammen
   rund 20 %) sind heterogene Ad-hoc-Skripte ohne feste Ausgabeform. Dafür ist keine
   Befehlsregel sinnvoll; höchstens eine generische Strategie (z. B. Wiederholungen
   zusammenfassen) in einem eigenen Issue.

Sobald tollgate eigene Phase-1-Daten mit Bash-Ausgabegrößen hat, sollte dieses Ranking
damit gegengeprüft werden.

# tollgate

Go-Gateway für alle Claude-API-Aufrufe aus Claude Code: Redact, Compress, Meter, Forward über
CLIProxyAPI, eingebettetes Svelte-Frontend zeigt jede Stufe. Alles passiert im Proxy; der
Client setzt nur URL und Client-Key. `AGENTS.md` ist ein Symlink hierauf; ergänzend
verbindlich: `docs/standards.md`, `docs/ops-lab.md`.

## Befehle

| Zweck | Befehl |
|---|---|
| Alle Tests inkl. Frontend (Projektprüfung) | `make test` |
| gofmt und vet | `make lint` |
| Binary mit Frontend bauen | `make web build` |
| Betrieb (Stack, Clients, Logs, Smoke) | `just --list` |

## Repo-Regeln
- Repo wird später **public**: keine Secrets, internen Hostnamen, Kundennamen oder
  ungeklärten Lizenzen ins Repo. `ref/` wird nie committet.
- Vor Kopieren oder Portieren LICENSE prüfen, Herkunft in `third_party/NOTICE.md`, Patches
  in `third_party/<name>/PATCHES.md` begründen. CLIProxyAPI bleibt ungepatcht (sonst neues ADR).
- `Makefile`, CI und Gate-Regeln sind Ein-Weg-Türen: ein Mensch liest den Diff vor dem Merge.
  Push, PR und Merge nur auf ausdrücklichen Auftrag (`docs/ops-lab.md`).
- Ein Issue = ein OpenSpec-Change auf eigenem GitButler-Branch im gemeinsamen Workspace.
  Keine Worktrees, kein `git checkout`/`switch`/`stash`, kein `but teardown`. Nur eigene
  Change-IDs committen (`but commit -b <branch> -m … <id>…`); fremde Änderungen nie anfassen.

## Harte Regeln
- Niemals Read-, Edit- oder Datei-Inhalte komprimieren, nur Bash-Ausgaben.
- Alle Transformationen sind deterministisch: gleiche Eingabe, Byte für Byte gleiche
  Ausgabe. Keine Zeitstempel, kein Zufall, keine Map-Iterationsreihenfolge. Sonst
  bricht das Prompt-Caching.
- Thinking-Blöcke und ihre Signaturen werden nie verändert. Nur User-Text und
  `tool_result` anfassen.
- Unbekannte Content-Typen und Felder werden unverändert durchgereicht. JSON wird so
  verarbeitet, dass nicht modellierte Felder nicht verloren gehen.
- In Logs, DB und Frontend landen nur redigierte Daten, nie Klartext-Geheimnisse. Einzige
  Ausnahme: „Bearbeiten“ der Kunden-, Vornamen- und Nachnamenliste nach ADR-0021 (Klartext
  nur auf ausdrücklichen Klick im Frontend).
- tollgate bindet nur an die Tailnet-IP, CLIProxyAPI nur an `localhost`. Im Container gilt
  das für den von Docker veröffentlichten Socket auf der Tailnet-IP des Hosts (ADR-0013).
- Im auth-dir von CLIProxyAPI liegt immer nur die Token-Datei des aktiven Accounts.
  Das Frontend zeigt an, welcher Account aktiv ist.

## Redactor-Port nach Go
- Übernimm alle Testfälle aus dem Rust-Repo als Go-Testdaten unter
  `internal/redact/testdata/`.
- Parität: Ein Differenztest lässt Rust- und Go-Version über denselben Korpus laufen.
  Abweichungen sind entweder Fehler oder werden im ADR als bewusste Änderung begründet.
  Der Test findet das Rust-Repo über `TOLLGATE_REDACTOR_REPO` (Default
  `ref/hook-redactor`) und wird übersprungen, wenn es dort nicht existiert.

## World-Pack

Auftrag, Pipeline, Redactor-Konfiguration, Frontend, Tests, OpenSpec- und GitButler-Ablauf:
`tollgate-world/world.md#coverage-map`, nur den Abschnitt lesen, den die Aufgabe braucht.

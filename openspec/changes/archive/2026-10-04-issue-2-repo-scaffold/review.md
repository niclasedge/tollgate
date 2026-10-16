# Unabhängiges Review

## Erstes Review (vor #34)

Spec und Standards ohne Befunde: Build, Startvertrag, Race-Testbefehl, gepinnte Generierung, CI und README entsprechen dem Change; elf Pakete und zehn Platzhalter stimmen mit ADR-0002, Go 1.26.5 und tygo v0.2.21 mit ADR-0003 überein.

## Zweites Review (nach Stapelung auf #34)

Anlass: Vertragstests nach Go portiert, Ops-lab-Belege entfernt. Spec ohne Blocker, alle Szenarien abgedeckt.

Behoben:
- Testcheckout kopiert die Dateiliste aus `git ls-files --cached --others --exclude-standard` statt geratener Verzeichnisnamen; lokale, ignorierte Dateien gelangen nicht mehr hinein.
- Testläufe setzen `GOFLAGS`, `GOOS` und `GOARCH` zurück, damit geerbte Umgebung den Build nicht verändert.
- Vergleich generierter Typen liest Unterverzeichnisse rekursiv.

Bewusst offen (Kleinigkeiten): kein eigener Test für Netzwerkfreiheit des Gerüsts (Programmeinstieg importiert nur `log/slog` und `os`), Starttest in `main_test.go` und Vertragstest überschneiden sich, kein `go vet`/`gofmt` in CI. Erfasst in #36.

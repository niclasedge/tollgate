# Tasks

## 1. Prüfbarer Startvertrag

- [x] 1.1 Zuerst ausschließlich die Vertragstests schreiben, `cmd/tollgate/main_test.go` (Go-Starttest) und `cmd/tollgate/contract_test.go` (Black-box `make build`/Start/Generator aus frischem Checkout): Das gebaute Binary startet ohne Konfiguration, schreibt genau einen parsebaren JSON-Logeintrag auf stderr, zeigt den Gerüststatus mit Level INFO, schreibt nichts auf stdout und endet mit Exit-Code 0. Gate 2 dokumentiert den roten Test vor Implementierung.
- [x] 1.2 Go-Modul `github.com/niclasedge/tollgate` mit Go 1.26.5 sowie minimalen Programmeinstieg implementieren; `cmd/tollgate/main_test.go` grün ausführen und Netzwerkfreiheit am Programmeinstieg prüfen.

## 2. Struktur und Werkzeuge

- [x] 2.1 Elf interne Pakete mit `doc.go`, die zehn Daten-/Ausgabeverzeichnisse mit Platzhaltern und `third_party/NOTICE.md` anlegen; `go list ./...` mit ADR-0002 vergleichen und `.gitignore` mittels `git check-ignore` auf Build-Ausgaben, Dist-Ausgaben, versionierbaren Platzhalter und unverändertes `ref/` prüfen.
- [x] 2.2 tygo v0.2.21 als Go-Tool pinnen, leere `tygo.yaml` und dokumentierten No-op für noch fehlende API-Typen bereitstellen; offizielle gepinnte Quelle, `go tool tygo --help`, stabiles `go mod tidy` und zweimalige identische Generierung prüfen.
- [x] 2.3 `make build`, `make test` und `make generate` samt README mit Voraussetzungen, Ausgabeorten, Gerüstumfang und Erweiterung in #9 erstellen; CGO-freien Binary-Build und Race-Testbefehl anhand tatsächlicher Läufe prüfen. Die dokumentierten Befehle und `cmd/tollgate/main_test.go` ausführen.

## 3. CI und Abschluss

- [x] 3.1 CI für Pull Requests und main-Pushes mit verifizierten Action-SHAs, Leserechten, Go-Version aus go.mod und allen drei Make-Befehlen erstellen; Generierungsprüfung auf veränderte versionierte und neue unversionierte Dateien durch temporäre Ausgaben in einem Branch-Export negativ prüfen.
- [x] 3.2 Frischen Branch-Export per `git archive issue-2/scaffold` ohne lokale Referenzen oder Zugangsdaten erstellen und alle Make-Befehle sowie den Binary-Start ausführen; Go-Formatierung und `openspec validate issue-2-repo-scaffold --strict` prüfen.
- [x] 3.3 Den tatsächlich umgesetzten Dateiumfang und die Review-Bereitschaft im Fortsetzungsstand dokumentieren; bestätigen, dass die unveränderten Vertragstests bestehen und keine Proxy-/Frontend-Funktion oder Fremdcodekopie vorweggenommen wurde.

## 4. Nacharbeit nach #34

- [x] 4.1 Node-Vertragstest `tests/scaffold.test.mjs` nach Go portiert (`cmd/tollgate/contract_test.go`), damit `make test` und CI ihn ausführen. Prüfen: Beide Tests werden rot bei zweiter Logzeile bzw. nicht stabiler Generierung (Mutationsprobe), sonst grün.
- [x] 4.2 Ops-lab-Belege (`belege/`, `seite/`, `messung/`, `teststand/`, `gate1.md`, `gate2.md`, `fertig.md`) und `docs/SESSION_HANDOFF.md` entfernt; Agent-Werkzeug liegt nach #34 nicht mehr im Repo.
- [x] 4.3 Auf `issue-34/repo-hygiene` gestapelt; `make test`, `make build`, `make generate`, `go mod tidy` und `openspec validate issue-2-repo-scaffold --strict` grün.
- [x] 4.4 Erneutes unabhängiges Review, PR mit `Closes #2` (#37), CI grün auf PR-HEAD, gemergt, archiviert.

# Tasks

## 1. jsonsplice

- [x] 1.1 Tests zuerst (`jsonsplice_test.go`, `fuzz_test.go`): Roundtrip, Zielauswahl nach Blocktabelle, gesperrte Blöcke, Fehlerfälle, Escape, Fuzz; rot bestätigt (Paket ohne Implementierung nicht übersetzbar).
- [x] 1.2 `Scan`, `Apply`, `Escape` implementieren (`go-json-experiment/json` per Pseudo-Version in `go.mod`); Tests mit `-race` grün.
- [x] 1.3 Fuzz-Lauf `FuzzApply` 30 s ohne Fund.

## 2. pipeline

- [x] 2.1 Tests zuerst (`pipeline_test.go`, `intake_test.go`): Roundtrip, Trace-Felder, kein Klartext im Trace, Trace-ID, Abbruch, Determinismus, Auth, Parsing, Sitzungsschlüssel; rot bestätigt.
- [x] 2.2 `Stage`, `Request`, `Runner`, `Trace`, `StageError`/`ErrorResponse` und `Intake` mit `KeyVerifier`/`ConfigPinner` implementieren; Tests mit `-race` grün.

## 3. Abschluss

- [x] 3.1 `gofmt -l`, `go vet`, `CGO_ENABLED=0 go build ./cmd/tollgate` grün; `openspec validate issue-5-pipeline-kern --strict` grün.
- [x] 3.2 Sitzungs-ID-Quelle: bewusst nach #41 ausgelagert (Aufzeichnung braucht den Proxy aus #6; #41 blockiert #13).
- [x] 3.3 Unabhängiges Review, PR mit `Closes #5`, nach Merge archivieren (Koordinator).

## 4. Review-Nacharbeit

- [x] 4.1 Runner lässt nur Ersetzungen zu, die exakt einem `Scan`-Ziel entsprechen (Signatur, Thinking, Member-Namen und Bereiche innerhalb eines Strings führen zu 500); Test `TestReplacementsOutsideTargetsAbort`.
- [x] 4.2 `Scan` baut Pfade in einem gemeinsamen Puffer: lineare statt quadratischer Speicher bei tiefer Verschachtelung; Test `TestScanDeepInputUsesLinearMemory` (vorher 89 MB bei 54 KB Body).
- [x] 4.3 Testlücke Typfilter geschlossen (Blöcke mit Feldern fremder Art in `sampleBody`).
- [x] 4.4 `Authorization` mit anderem Schema fällt auf `x-api-key` zurück.
- [x] 4.5 Trennbyte im Sitzungsschlüssel als ADR-0011 festgehalten, `docs/ARCHITECTURE.md` angepasst.

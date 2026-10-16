# Tasks

## 1. Tests zuerst

- [x] 1.1 `internal/auth/auth_test.go`: Key-Format, Verifier-Lebenszyklus (Sperren, Rotieren, fehlerhafte Keys, Store-Fehler), Admin-Token, API-Fehler, Liste ohne Key, Leck-Test über DB-Dateien und Logs, Token-Datei, Fuzz. Rot vor der Umsetzung bestätigt (Paket kompilierte nicht).

## 2. Umsetzung

- [x] 2.1 `key.go`, `verifier.go`, `types.go`, `admintoken.go`, `adminapi.go`; Tests mit `-race` grün, `gofmt`, `go vet`, Fuzz-Lauf ohne Fund.

## 3. Abschluss

- [x] 3.1 `openspec validate issue-8-client-keys --strict` grün; `CGO_ENABLED=0 go build ./cmd/tollgate` grün.
- [x] 3.2 Verdrahtung (Verifier in `Intake`, Admin-API unter `/api/clients`, Token-Datei, `tygo.yaml`) erfolgt mit #6, das `cmd` und `internal/server` besitzt; dieser Change liefert die Bausteine.

## 4. Review-Nacharbeit

- [x] 4.1 Bestehende Admin-Token-Datei muss eine reguläre Datei ohne Gruppen-/Weltrechte sein; Test `TestAdminTokenFileMustBePrivateRegularFile`.
- [x] 4.2 Rotate liest den Client über die ID zurück (keine irreführende 404 bei parallelen Rotationen).
- [x] 4.3 Daten nach dem JSON-Wert ergeben 400, zu große Bodies 413.
- [x] 4.4 Test: leeres Admin-Token öffnet nie die API.

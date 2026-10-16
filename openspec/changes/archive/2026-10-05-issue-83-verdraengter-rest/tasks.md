# Tasks

## 1. Tests zuerst

- [x] 1.1 Korpusfall `internal/redact/testdata/corpus/tollgate/residual_of_displaced_generic.txtar` (want, must_not_contain, Abweichung `overlap`).
- [x] 1.2 `TestResolution`: Rest nach JWT, Rest zwischen zwei E-Mails, Rest ohne Buchstabe/Ziffer bleibt, Schlüsselname eines anderen Detektors bleibt.
- [x] 1.3 `TestAllowlistWins`: Rest in der Allowlist bleibt.
- [x] 1.4 Rot sehen (Gate 2).

## 2. Umsetzung

- [x] 2.1 `Find`: Kandidaten merken ihren Nicht-Secret-Teil; verdrängte generische Kandidaten liefern Restspannen.
- [x] 2.2 `parity_test.go`: Beschreibung von `overlap` ergänzen.
- [x] 2.3 ADR-0005: Nachtrag #83.

## 3. Abschluss

- [x] 3.1 `go test -race ./internal/redact/`, Paritätstest mit `TOLLGATE_REDACTOR_REPO`, `make lint`, `make test`.
- [x] 3.2 `openspec validate issue-83-verdraengter-rest --strict`.

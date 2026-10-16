## 1. Tests zuerst
- [x] 1.1 `internal/store/calls_test.go`: Grenzen `from` inklusiv, `to` exklusiv, leerer Bereich.
- [x] 1.2 `internal/dashboard/calls_test.go`: tabellengetrieben über Fenstergrenzen, leere Fenster, `above`/`below`/`equal`, Durchschnitt mit einer Nachkommastelle, 401 ohne Admin-Token.
- [x] 1.3 `web/src/calls.test.ts`: Zahl, Ø, Indikator „über Ø“/„unter Ø“/„wie Ø“, Fehler im Abschnitt, Neuladen per „Aktualisieren“.
- [x] 1.4 Neue Tests sind vor der Umsetzung rot.

## 2. Umsetzung
- [x] 2.1 `Store.CountTraces`.
- [x] 2.2 Typ `CallRate`, Route `GET /api/calls`, `make generate`.
- [x] 2.3 `api.calls`, `CallRate.svelte`, Abschnitt in `Home.svelte`.

## 3. Abschluss
- [x] 3.1 `go test -race ./...`, `make lint`, `pnpm check`, `pnpm test` und `openspec validate issue-132-calls-5min --strict` grün.

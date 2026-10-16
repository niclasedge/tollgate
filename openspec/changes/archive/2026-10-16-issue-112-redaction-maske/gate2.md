# Gate 2
Branch `issue-112/redaction-maske`, gemeinsamer Workspace (geprüft wurde die Kombination aller angewendeten Branches und der uncommitteten Arbeit im Workspace).

## Roter Vorlauf
Vor der Umsetzung (Feld `Mask` in den Typen vorhanden, `redact.Mask` als Stub mit `""`, keine Store-Prüfung, kein Mapping, keine Anzeige); Log lokal unter `/tmp/tollgate-issue112-red.log`:

- `internal/redact`: `TestMask` mit allen 25 damaligen Fällen rot (zwei E-Mail-Fälle kamen beim Nachrechnen der Viertelgrenze hinzu, heute 27) (`Mask = "", want "n•••@f•••.de"` usw.), `TestMaskInvariantsOnFindings` rot, `TestStageMasks` rot (`match 0 mask = ""`).
- `internal/store`: `TestMatchMaskGrammar` rot (`"geheim123": err = <nil>, want ErrInvalid`, ebenso E-Mail, Steuerzeichen, Überlänge, ungültiges UTF-8), `TestRecentRedactions` rot (`Mask:` leer), `TestStoredTypesHaveNoPlaintextFields` rot (neues Feld `Mask`, Allowlist bewusst ergänzt).
- `internal/dashboard`: `TestTraceDetail`, `TestRecentRedactionsEndpoint` und `TestNoPlaintextInDetailAndRedactions` rot (`detail match 0 mask = ""`, `redaction <redacted:email_1> without mask`).
- `cmd/tollgate`: `TestNoPlaintextInDBLogsAndAdminAPI` rot (`hit <redacted:word_1> (word) without mask: ""`, `mask "" of <redacted:connection_password_1> shows clear characters`).
- `web/`: `pnpm exec vitest run src/timeline.test.ts src/home.test.ts`: 3 von 17 Tests rot (keine Tabelle „Treffer der Stufe Redact“, kein Element „Maske“).

## Grün nach der Umsetzung (2026-10-05)
- `TOLLGATE_REDACTOR_REPO=$HOME/src/hook-redactor make test`: `pnpm check` 0 Fehler/0 Warnungen, `pnpm test` 13 Dateien/123 Tests grün, `go test -race ./...` alle Pakete `ok`; `TestParityWithRust` läuft gegen das Rust-Repo und ist grün.
- `make lint`, `make generate` (nur `mask` in `web/src/lib/types/dashboard.ts`), `make web`, `make build` grün.
- `openspec validate issue-112-redaction-maske --strict` grün.
- Screenshots `.claude/belege/issue112/{timeline,home}-{desktop,iphone}-{hell,dunkel}.png` gegen `bin/tollgate` (127.0.0.1, `--dev-allow-any-listen`, Temp-Verzeichnis), Mock-Upstream, erfundene Daten.
- Kein Commit, Push oder PR.

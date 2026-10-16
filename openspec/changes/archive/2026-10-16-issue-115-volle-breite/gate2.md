# Gate 2
Branch `issue-115/volle-breite`, gemeinsamer Workspace.

## Roter Vorlauf
Vor der Umsetzung `pnpm exec vitest run src/layout.test.ts` in `web/`: 3 von 3 Tests rot (Log lokal `/tmp/tollgate-issue115-red.log`):
- `content area has no fixed max-width`: `main` enthält `max-width: 64rem`.
- `subscription tiles fit three in a row at 1440px`: `expected 73.8 to be less than or equal to 68.5` (Mindestbreite 24rem).
- `prose paragraphs stay readable`: Regel für Absätze fehlt.
Die Absatzregel wurde danach von `app.css` nach `Shell.svelte` verlegt (Vitest liefert `.css?raw` leer); gegen den Ausgangsstand ist der Test ebenso rot (Regel fehlt).

## Validierung
- `pnpm exec vitest run src/layout.test.ts`: 3 von 3 grün.
- `make test`: svelte-check 0 Fehler, 0 Warnungen; Vitest 14 Dateien, 126 Tests grün; `go test -race ./...` grün.
- `make lint` grün, `make build` grün, `vite build` grün.
- `openspec validate issue-115-volle-breite --strict` grün.
- Screenshots mit erfundenen Daten (alle `/api`-Aufrufe im Browser gemockt, eigener Vite-Dev-Server auf Port 5199, danach beendet) unter `.claude/belege/issue115/`: Übersicht bei 1440 px, 1920 px und 390 px, hell und dunkel; zusätzlich Live, Kosten, Accounts, Filter, Clients bei 1920 px und 390 px. Messung: drei Abo-Kacheln in einer Zeile bei 1440 px (`main` 1224 px) und 1920 px, untereinander bei 390 px, kein horizontaler Überlauf.
- Kein Commit, Push oder PR.

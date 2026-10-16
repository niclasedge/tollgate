# Design

## Embedding ohne Frontend-Build

`web/dist` enthält im Checkout nur `.gitkeep` (`.gitignore` schließt den Rest aus). `//go:embed all:dist` kompiliert damit immer. `Handler()` prüft, ob `dist/index.html` existiert; falls nicht, liefert er eine eingebettete Platzhalterseite (`web/placeholder.html`).

Verworfen: ein eingechecktes `dist/index.html` – es würde nach `make web` als Änderung erscheinen oder die `.gitignore`-Regel aus #2 aufweichen; außerdem würde der CI-Check „keine veränderten Dateien“ brechen.

## Make-Ziele

- `make build` bleibt Go-only. Der Vertragstest `cmd/tollgate/contract_test.go` baut aus frischem Checkout und darf weder Node noch Netzwerk brauchen.
- `make web` installiert mit `--frozen-lockfile`, leert `dist/*` (ohne Dotfiles, `.gitkeep` bleibt) und baut. Vite läuft mit `emptyOutDir: false` aus demselben Grund.
- `make test` = `web-test` (install, `svelte-check`, `vitest`) und danach `go test -race ./...`.
- CI: `setup-node` 22, `corepack enable` (pnpm-Version aus `packageManager` in `web/package.json`), `make test`, `make web`, `make build`.

## Router, Tastatur, Theme

- Hash-Router (~40 Zeilen) mit sieben Bereichen und optionalem Detailsegment (`#/requests/<id>`) für Inline-Details; unbekannte Hashes fallen auf Live zurück. Kein SPA-Fallback im Server nötig, unbekannte Pfade sind 404.
- Tastatur: `1`–`7` wechselt den Bereich, `d` den Modus; ignoriert in Eingabefeldern und mit Modifikatoren. Skip-Link, sichtbare Fokusringe, `aria-current`.
- Theme: ohne Wahl gilt `prefers-color-scheme` rein per CSS; eine Wahl wird in `localStorage` gespeichert und per Inline-Skript vor dem ersten Paint gesetzt.

## SSE-Client

Dünner Wrapper um `EventSource` (injizierbar): benannte Events, JSON-Dekodierung (Fallback String), Status-Callback. Solange der Browser selbst neu verbindet (`readyState` CONNECTING), greift der Client nicht ein; bei CLOSED Neuverbindung mit Backoff 1 s → max. 30 s, Reset nach erfolgreichem Open.

## Typen und tygo

Es gibt noch keine API-Structs, `tygo.yaml` bleibt unverändert (`packages: []`, `make generate` bleibt No-op, deterministisch, ohne globale Installation). Zielordner `web/src/lib/types/` bleibt. Die erste API-Struktur ergänzt das Paket in `tygo.yaml`.

## Build-Detail

`resolve.conditions: ['browser']` ist nötig: Mit Vite 8 löst der Build sonst die Server-Variante von Svelte auf (`mount(...) is not available on the server`). TypeScript ist auf 6.x gepinnt, weil `svelte-check`/Plugin 7.x noch nicht als Peer akzeptieren.

## Design

Eigene Tokens in `src/app.css`: ein Blauton (`--c-accent`), sonst Grautöne, kleine Radien, 150 ms Übergänge (aus bei `prefers-reduced-motion`). Sidebar 4,25 rem mit Icons; unter 720 px Bottom-Leiste mit Safe-Area. Bausteine für Folge-Issues: `Disclosure` (Inline-Detail), `StatusDot` (runder Statusindikator), `EmptyView`. Icons sind eigene Strichgrafiken, keine Fremd-Assets.

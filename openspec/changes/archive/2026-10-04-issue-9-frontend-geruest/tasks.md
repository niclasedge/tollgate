# Tasks

## 1. Tests zuerst

- [x] 1.1 Vitest-Tests für SSE-Client (Mock-EventSource), Router, App-Shell sowie Go-Tests für `Handler()` geschrieben; vor der Umsetzung rot (fehlende Module bzw. `undefined: Handler`).

## 2. Umsetzung

- [x] 2.1 Svelte-5-Gerüst, Tokens, Dark Mode, Router, Tastatur, Icons, Basisbausteine.
- [x] 2.2 `web/embed.go` mit Platzhalter-Fallback.
- [x] 2.3 `make web`, `make web-test`, `make test`; CI mit Node/Corepack und Frontend-Build.

## 3. Prüfung

- [x] 3.1 Screenshots Desktop/Mobil, hell/dunkel unter `/tmp/tollgate-shell-*.png`.
- [x] 3.2 `go test -count=1 ./cmd/tollgate/` (frischer Checkout) grün.
- [x] 3.3 `openspec validate issue-9-frontend-geruest --strict` grün.
- [ ] 3.4 Handler in den Server verdrahten (Koordinator nach #6).

## 4. Review-Nacharbeit

- [x] 4.1 `parseHash`: kaputtes Escape im Detail-Segment wirft nicht mehr (Test).
- [x] 4.2 `packageManager` mit sha512-Hash gepinnt.
- [x] 4.3 Embed-Handler: Verzeichnisse und Dotfiles liefern 404 statt Listing/Redirect (Test).
- [x] 4.4 Skip-Link fokussiert `main`, ohne die Route zu ändern (Test).

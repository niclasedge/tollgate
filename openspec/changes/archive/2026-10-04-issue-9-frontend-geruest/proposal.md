# Proposal

Issue: [#9 Frontend-Grundgerüst: Svelte, tygo, embed, Design-System](https://github.com/niclasedge/tollgate/issues/9)

## Why

Die Frontend-Issues #10 und #16 brauchen ein gemeinsames Gerüst: Build, Embedding im Binary, Navigation, Design-Tokens, Live-Daten-Client und Tests. Ohne dieses Gerüst würde jedes Issue es neu erfinden.

## What Changes

- Svelte-5-App (Vite, TypeScript strict, pnpm) unter `web/` mit leerer Shell: schmale Icon-Sidebar, sieben Bereiche (Live, Anfragen, Kosten, Filter, Kompression, Clients, Accounts), eigener Hash-Router.
- Eigene CSS-Tokens, ein Blau-Akzent, Dark Mode (System und manuell, gespeichert), responsiv (Bottom-Tab-Leiste auf dem Handy), Tastatursteuerung (1–7, `d`), eigene SVG-Icons.
- SSE-Client-Modul mit Reconnect-Backoff, getestet gegen einen Mock-EventSource.
- Go-Paket `web` mit `Handler()`, das `web/dist` per `embed` ausliefert; Platzhalterseite, solange `dist` nicht gebaut ist.
- `make web`, `make web-test`; `make test` führt die Frontend-Tests zuerst aus; CI baut das Frontend vor dem Binary.

## Capabilities

### New Capabilities

- `frontend-shell`: Eingebettete, navigierbare Frontend-Shell mit Design-System, SSE-Client und Testlauf in `make test`.

### Modified Capabilities

Keine.

## Impact

- Neues Verzeichnis `web/` mit Node-Abhängigkeiten (nur Dev-Abhängigkeiten, gepinnt in `pnpm-lock.yaml`); keine neue Go-Abhängigkeit.
- `make test` und CI benötigen Node 22 und pnpm (Corepack, Version aus `web/package.json`). `make build`, `make generate` und `go test` benötigen sie weiterhin nicht.

## Was ausdrücklich nicht

- Keine Verdrahtung des Handlers im Server (Koordinator nach #6), keine API-Aufrufe, keine Fachansichten (#10, #16), kein CodeMirror (kommt mit #16), keine API-Typen (`tygo.yaml` bleibt leer, es gibt noch keine API-Structs; Typen entstehen mit #6/#8/#10).
- Keine Logos, Icons, Namen oder Assets von Things 3.

## Berührte Dateien

- `web/**` (Konfiguration, `src/`, `embed.go`, `embed_test.go`, `placeholder.html`, `pnpm-lock.yaml`)
- `Makefile`
- `.github/workflows/ci.yml`
- `openspec/changes/issue-9-frontend-geruest/**`

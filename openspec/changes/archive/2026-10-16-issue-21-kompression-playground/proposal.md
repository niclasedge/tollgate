# Proposal

Issue: [#21 Frontend Phase 3: Kompressions-Playground](https://github.com/niclasedge/tollgate/issues/21)

## Why

Der Bereich „Kompression“ ist noch ein Platzhalter. Man sieht weder, welche Regel für
welchen Befehl greift, noch was die Strategien mit einer Ausgabe machen. ADR-0007 sieht
dafür den Trace der Strategieschritte vor (`strat.Trace`), der bisher ungenutzt ist.

## What Changes

- `compress.Config.Explain(command, output)`: dieselben Gates wie `Compress`, dazu der
  Text nach jedem Schritt (Eingabe, Schritte der Strategie, Ergebnis mit Markerzeile)
  und ein Grund, wenn die Ausgabe unverändert bleibt. Ergebnis und Entscheidung sind
  identisch mit `Compress` (Test).
- `GET /api/compress/rules`: Regeln der aktiven Version in Prüfreihenfolge, eigene und
  eingebaute markiert, dazu `min_bytes` und ob die Stufe läuft.
- `POST /api/compress/playground`: Befehl und Ausgabe gegen die aktive (oder eine
  angegebene) Version. Text wird weder gespeichert noch geloggt.
- Neue Ansicht „Kompression“: Regelliste und Playground. Die Schritte klappen inline
  auf, das Ergebnis ist offen.
- TS-Typen aus `internal/compress/types.go` (tygo).

## Capabilities

### Modified Capabilities

- `compression`: Erklärung jedes Schritts für den Playground.
- `frontend-views`: Bereich „Kompression“.

## Impact

- Backend: `internal/compress` (`types.go`, `explain.go`, `Rule.Builtin`),
  `internal/redactcfg` (Routen, `Options.CompressEnabled`), `cmd/tollgate/main.go`.
- Frontend: `views/Compression.svelte`, `App.svelte`, `lib/api.ts`, generierte Typen.
- Abhängigkeit: #19 (Compress-Stufe) ist lokal umgesetzt, aber noch offen; dieser Change
  liegt im Stack darüber.

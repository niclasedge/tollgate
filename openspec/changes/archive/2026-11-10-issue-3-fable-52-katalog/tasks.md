# Tasks

## 1. Umsetzung

- [x] 1.1 `TestTollgateOverlay`: jeder Overlay-Eintrag genau einmal, eigenes Thinking-Objekt, statischer Lookup, idempotent; Fable 5.2 nur adaptiv.
- [x] 1.2 `models_overlay.go.txt` als Tabelle mit `claude-fable-5-2`; Dockerfile führt `TestTollgateOverlay` aus.
- [x] 1.3 Meter-Preis `claude-fable-5-2` mit Test.
- [x] 1.4 ADR-0020-Nachtrag, `PATCHES.md`, `NOTICE.md`, `UPSTREAM.md`.

## 2. Abschluss

- [x] 2.1 `make test`, `make lint` grün; Image neu gebaut und deployt; Request mit `claude-fable-5-2` liefert kein 400.

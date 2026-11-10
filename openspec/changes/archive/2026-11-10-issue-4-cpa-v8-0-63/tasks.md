# Tasks

## 1. Prüfung (UPSTREAM.md)

- [x] 1.1 go.mod ohne replace, LICENSE unverändert MIT.
- [x] 1.2 Claude-Erkennung, Cloaking, Body-Eingriffe, Antwortpfad.
- [x] 1.3 Management-API (credentials), Refresh-Barriere-Einfügestellen.
- [x] 1.4 config.example.yaml, neue Defaults, ausgehende Ziele, `-local-model`, `/healthz`.

## 2. Umsetzung

- [x] 2.1 Pin, Overlay entfernen, Config-Pins, ADR-0022, Doku.

## 3. Abschluss

- [x] 3.1 `make cpa` + Smoke mit echtem Binary, `make test`, `make lint`, Image-Build.
- [x] 3.2 Deploy; Claude Code mit `claude-fable-5-2` und `claude-haiku-5-5` ok; Cache-Reads im Folgerequest.

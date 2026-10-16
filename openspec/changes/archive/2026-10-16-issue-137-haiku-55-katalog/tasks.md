# Tasks

## 1. Umsetzung

- [x] 1.1 `TestTollgateHaiku55` zuerst: Haiku 5.5 genau einmal gelistet, Werte wie Sonnet 5.5, eigenes Thinking-Objekt, statischer Lookup, idempotent.
- [x] 1.2 `models_overlay.go.txt` klont `claude-sonnet-5-5` als `claude-haiku-5-5`; `apply.sh` kopiert Overlay und Test; Dockerfile führt den Test aus.
- [x] 1.3 ADR-0020, `third_party/cpa-model-overlay/PATCHES.md`, `NOTICE.md`, `UPSTREAM.md`.

## 2. Abschluss

- [x] 2.1 `make test`, `make lint` grün; Image neu gebaut; Speedtest mit `claude-haiku-5-5` liefert 200.

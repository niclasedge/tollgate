# Tasks

## 1. Umsetzung

- [x] 1.1 Tests: Gate (`Force` nur bei Vormerkung), Manager (erzwungener Wechsel aktiviert), Proxy (503 + `account_switch`), Frontend (Button in Verwaltung und Übersicht).
- [x] 1.2 `accounts.Gate`: abbrechbarer Kontext je Anfrage, `Force`; `Manager.ForceSwitch`.
- [x] 1.3 Proxy: Anfrage-Kontext aus dem Gate, 503 bei erzwungenem Abbruch.
- [x] 1.4 API `POST /api/accounts/switch/force`, Frontend-Button.

## 2. Abschluss

- [x] 2.1 `make test`, `make lint` grün; deployt (Mac mini, 2026-10-15).

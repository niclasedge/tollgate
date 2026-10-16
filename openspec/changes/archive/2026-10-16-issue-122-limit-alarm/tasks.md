# Tasks

## 1. Tests zuerst

- [x] 1.1 `internal/alert`: Watcher (Schwelle, ein Punkt eine Nachricht, Sprung, neues Fenster, Ablehnung per Header und per 429 an der Schwelle, 429 ohne Vorgeschichte und unter der Schwelle, Cooldown, kein Kandidat, Wechsel aus, Abo nicht mehr aktiv), Kandidatenwahl, Spool (atomar, Reihenfolge, Fehler, Alter, unlesbar, Slack aus), Slack-Client gegen `httptest`. Rot sehen.
- [x] 1.2 `internal/config`: Defaults, YAML, Umgebung, ungültige Schwelle und ungültiger Bool.
- [x] 1.3 `cmd/tollgate`: `tollgate notify` liefert den Spool an ein Fake-Slack, ohne Token verwirft er, ohne `--spool` startet er nicht.
- [x] 1.4 `deploy`: Compose-Dienst `notifier` und Volume, Secrets nur beim Notifier.

## 2. Umsetzung

- [x] 2.1 `internal/alert` (watch, candidate, spool, slack).
- [x] 2.2 `internal/config`: Abschnitt `alerts`.
- [x] 2.3 `internal/accounts`: Hook `OnLimits`.
- [x] 2.4 `cmd/tollgate`: `startAlerts` (Hook und Broker-Abo), Unterbefehl `notify`.
- [x] 2.5 Deployment: Dockerfile, Compose, `setup.sh`, `smoke.sh`, `justfile`.
- [x] 2.6 ADR-0019, `docs/DEPLOY.md`, `docs/ARCHITECTURE.md`.

## 3. Abschluss

- [x] 3.1 `make test` und `make lint` grün (CI auf main, 2026-10-16).
- [x] 3.2 `openspec validate issue-122-limit-alarm` grün (`--strict` meldet nur Längen-Warnungen).

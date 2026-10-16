# Proposal

Issue: [#122 Limit-Alarm per Slack und automatischer Account-Wechsel](https://github.com/niclasedge/tollgate/issues/122)

## Why

Läuft das 5-Stunden-Fenster des aktiven Abos voll, merkt man das erst, wenn Claude Code
Anfragen ablehnt. tollgate kennt die Auslastung aus jeder Antwort (Header
`anthropic-ratelimit-unified-*`) und die übrigen Abos im Tresor (#28, #29). Es kann also
vorher warnen und bei einer Ablehnung selbst auf ein Abo mit freiem Kontingent wechseln.

## What Changes

- **Warnung:** Ab 90 % im 5-Stunden-Fenster des aktiven Abos schreibt tollgate je höherem
  vollem Prozentpunkt genau eine Slack-Nachricht. Sie nennt das beste andere Abo mit freiem
  Wochenlimit, dessen 5-Stunden-Fenster unbekannt ist oder noch Platz hat, oder sagt, dass
  es keines gibt. Ein neues Fenster (anderer Reset) setzt den Zähler zurück.
- **Automatischer Wechsel:** Lehnt Anthropic eine Anfrage ab (Header-Status `rejected`,
  oder HTTP 429, nachdem das Abo zuletzt an der Warnschwelle lag; CLIProxyAPI reicht die
  Limit-Header bei 429 nicht weiter), wechselt tollgate
  über den vorhandenen Account-Wechsel auf dieses Abo und meldet das Ergebnis in Slack.
  Ohne freies Abo kein Wechsel, nur eine Meldung. Zwischen zwei Versuchen liegen mindestens
  10 Minuten.
- **Zustellung (ADR-0019):** tollgate legt Nachrichten nur als Dateien in ein
  Spool-Verzeichnis. Ein neuer Compose-Dienst `notifier` (gleiches Image, Befehl
  `tollgate notify`) liest sie und schickt sie per `chat.postMessage` an Slack. Die
  Egress-Sperre aus ADR-0017 bleibt unverändert, tollgate sieht den Slack-Token nie.
- **Konfiguration:** `alerts.spool_dir`, `alerts.auto_switch` (Default an),
  `alerts.five_hour_threshold` (Default 90) mit `TOLLGATE_ALERTS_*`. Slack-Token und
  Kanal sind Docker-Secrets nur des Notifiers; `just slack-secret` schreibt sie aus
  `SLACK_BOT_TOKEN` und `SLACK_ALERT_CHANNEL`.

## Capabilities

### New Capabilities

- `limit-alerts`: Warnung, automatischer Wechsel, Spool und Notifier.

## Impact

- Neu: `internal/alert`, `cmd/tollgate/notify.go`, ADR-0019.
- Geändert: `internal/config` (Abschnitt `alerts`), `internal/accounts` (Hook `OnLimits`),
  `cmd/tollgate/main.go`, `docker-compose.yml`, `deploy/setup.sh`,
  `justfile`, `deploy/tollgate/Dockerfile`, `docs/DEPLOY.md`, `docs/ARCHITECTURE.md`.

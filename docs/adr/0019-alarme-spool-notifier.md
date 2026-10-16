# ADR-0019: Alarme über Spool und Notifier-Container

- **Status:** Angenommen
- **Datum:** 2026-10-05
- **Issue:** #122
- **Ergänzt:** [ADR-0017](0017-cpa-egress-nur-anthropic.md) (Egress bleibt unverändert)

## Kontext

tollgate soll vor dem 5-Stunden-Limit warnen und Wechsel melden, per Slack
(`chat.postMessage` auf `slack.com`). tollgate läuft im Netz-Namespace von `cliproxyapi`.
Dort erlaubt ADR-0017 nur `tcp/443` zu Anthropic; `slack.com` ist nicht erreichbar.

## Entscheidung

1. **tollgate schreibt nur Dateien.** Nachrichten landen atomar als JSON-Dateien in einem
   Spool-Verzeichnis (`alerts.spool_dir`, im Container das Volume `tollgate-alerts` unter
   `/alerts`). tollgate braucht dafür kein Netz und keinen Slack-Token.
2. **Eigener Dienst `notifier`.** Gleiches Image, Befehl `tollgate notify`, im normalen
   Compose-Netz mit freiem Egress, als uid 10002 ohne zusätzliche Capabilities und ohne
   Ports. Er liest das Spool-Verzeichnis, sendet an Slack und löscht nach Erfolg.
3. **Secrets nur beim Notifier.** `slack_bot_token` und `slack_channel` sind
   Docker-Secrets, die nur der Notifier bekommt. Leere Dateien schalten Slack ab.

## Alternativen

- **Slack-CIDRs in die Egress-Regeln:** Slack liegt hinter wechselnden CDN-Adressen; die
  Liste wäre groß und instabil, und der Namespace mit den OAuth-Token bekäme einen zweiten
  Ausgang.
- **tollgate in ein eigenes Netz:** Bricht ADR-0013 (tollgate erreicht CPA nur über
  `127.0.0.1`).
- **HTTP-Proxy im Compose-Netz:** Mehr bewegliche Teile als ein Verzeichnis, und tollgate
  bräuchte doch den Token.

## Konsequenzen

- Ein Dienst mehr; fällt er aus, stauen sich höchstens 500 Dateien, ältere als 24 h
  verwirft er beim nächsten Lauf.
- Nachrichten kommen mit bis zu einem Poll-Intervall (10 s) Verzögerung.
- Der automatische Wechsel hängt nicht am Notifier; er läuft auch ohne Slack.

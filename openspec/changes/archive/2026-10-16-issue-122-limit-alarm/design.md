# Design

Grundlage: `meter.ParseLimits` (Header `anthropic-ratelimit-unified-*`), Account-Tresor und
Wechsel (#28, #29), ADR-0017 (Egress nur Anthropic). Entscheidung zur Zustellung:
[ADR-0019](../../../docs/adr/0019-alarme-spool-notifier.md).

## 1. Paket `internal/alert`

| Datei | Inhalt |
|---|---|
| `watch.go` | `Watcher.Observe(accountID, status, header)` und ein Worker |
| `candidate.go` | Kandidatenwahl und Texte |
| `spool.go` | `Spool.Write(text)`, `Spool.Drain(ctx, send, now)` |
| `slack.go` | `Slack.Send(ctx, text)` gegen `chat.postMessage` |

`Observe(id, meter.Limits)` hängt am neuen Hook `accounts.Manager.OnLimits` (wie `OnSwitch`),
der in `ObserveLimits` mit den bereits geparsten Limits aufgerufen wird. `ObserveStatus(id,
status)` liest der Watcher als Abonnent des Trace-Brokers (`dashboard.Broker`), der jede
fertige Anfrage mit Status und `account_id` verteilt. So bleiben `internal/server` und die
Observer-Signaturen unverändert. Beide laufen im Proxy-Pfad bzw. direkt dahinter: Zustand unter einer Mutex prüfen, höchstens
einen Auftrag in einen Kanal (Kapazität 16) legen. Ist der Kanal voll, wird der Auftrag
verworfen und geloggt. Ein einziger Worker (`Run(ctx)`) arbeitet die Aufträge in Reihenfolge
ab; so stimmt die Reihenfolge in Slack, und HTTP-Abfragen der Nutzung (`Usage`) oder der
Wechsel (`Activate`, wartet auf laufende Anfragen) blockieren nie eine Anfrage.

Zustand je Abo: Reset und Prozentpunkt der letzten Warnung, `near` (letzte Beobachtung
abgelehnt oder ein Fenster an der Warnschwelle). Global: Zeit des letzten Wechselversuchs und ob einer
läuft. Der Cooldown (10 min) zählt ab dem Einreihen des Versuchs, auch wenn kein Kandidat
frei war, damit eine Ablehnungsserie nicht jede Anfrage eine Nachricht erzeugt.

Prozentpunkt: `floor(u·100 + 1e-9)`, damit `0.29·100` nicht auf 28 fällt.

## 2. Ablehnung erkennen

CPA macht aus Anthropics 429 einen eigenen Fehler ohne Header
(`classifyClaudeUpstreamErrorWithCooling` → `statusErr`, am Pin `8ef43e4`); trotz
`passthrough-headers: true` kommen die Limit-Header bei 429 also nicht an. Danach legt CPA
das Abo in einen Cooldown und antwortet selbst mit 429. Header-Status `rejected` bleibt als
Signal erhalten, falls er doch ankommt. Hauptweg ist der Status: Eine 429 zählt als
Ablehnung, wenn die letzte Beobachtung des Abos abgelehnt war oder ein Fenster an der
Warnschwelle lag (`near`). Eine 429 ohne diese Vorgeschichte bleibt unbeachtet
(kurzzeitiges Rate-Limit, kein Abo-Limit).

## 3. Kandidaten

Quelle ist `accounts.Manager` über ein kleines Interface (`List`, `Usage`, `Activate`).
`Usage` liefert für inaktive Abos den zwischengespeicherten Stand (5 min) oder fragt die
Nutzungs-API mit dem Tresor-Token ab; ein Fehler zählt als „unbekannt“. Auslastungen sind
dort Prozent. Regeln und Rangfolge stehen in der Spec. Der Worker prüft vor dem Wechsel, ob
das abgelehnte Abo noch aktiv ist; sonst hat schon jemand gewechselt.

## 4. Texte

Slack-mrkdwn, deutsch, ohne E-Mail-Adressen, nur Abo-Namen:

- `⚠️ 5-h-Limit von *A* bei 91 % (Reset in 1 h 20 min). Ausweichen: *C* (Woche 40 %, 5 h unbekannt).`
- `🔁 *A* abgelehnt (5-h-Limit). Gewechselt auf *C* (Woche 40 %, 5 h 0 %).`
- `❌ *A* abgelehnt (5-h-Limit). Wechsel auf *C* fehlgeschlagen: <Fehler>`
- `⛔ *A* abgelehnt (Wochenlimit). Kein anderes Abo frei.`

## 5. Spool und Notifier

Dateiname `<UnixNano 20-stellig>-<Zähler 6-stellig>.json`, Inhalt
`{"text":"…","created":"RFC3339"}`. Schreiben: `os.CreateTemp(dir, ".tmp-*")`, Mode 0600,
dann `os.Rename`. Mehr als 500 Dateien: neue Nachricht verwerfen (Notifier hängt).

`tollgate notify --spool DIR --token-file F --channel-file F [--interval 10s] [--slack-url U]`
läuft bis SIGTERM. Token und Kanal werden bei jedem Durchlauf gelesen, damit
`just slack-secret` ohne Neustart greift.

## 6. Konfiguration

```yaml
alerts:
  spool_dir: /alerts          # TOLLGATE_ALERTS_SPOOL_DIR, leer: keine Nachrichten
  auto_switch: true           # TOLLGATE_ALERTS_AUTO_SWITCH
  five_hour_threshold: 90     # TOLLGATE_ALERTS_FIVE_HOUR_THRESHOLD, 1..100
```

Ohne Account-Verwaltung (kein Management-Key) gibt es keinen Watcher.

## 7. Deployment

- Image: `/alerts` (tollgate:tollgate, 0700) anlegen; ein neues benanntes Volume
  `tollgate-alerts` übernimmt diese Rechte beim ersten Einhängen.
- `tollgate`: Volume `/alerts`, `TOLLGATE_ALERTS_SPOOL_DIR: /alerts`.
- `notifier`: gleiches Image, `user: 10002:10002`, `entrypoint: ["tollgate", "notify", …]`,
  eigenes Default-Netz, Secrets `slack_bot_token` und `slack_channel`, Healthcheck aus,
  keine Ports, keine Capabilities.
- `deploy/setup.sh` legt leere Secret-Dateien an, `setup.sh slack` (Rezept
  `just slack-secret`) schreibt sie aus der Umgebung, ohne sie auszugeben.

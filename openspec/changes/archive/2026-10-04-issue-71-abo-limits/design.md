# Design

## Header

Anthropic sendet bei Abo-Anfragen (OAuth) mit jeder Antwort, auch bei 429:

| Header | Wert |
|---|---|
| `Anthropic-Ratelimit-Unified-Status` | `allowed`, `allowed_warning`, `rejected` |
| `Anthropic-Ratelimit-Unified-Reset` | Unix-Sekunden |
| `Anthropic-Ratelimit-Unified-Representative-Claim` | z. B. `five_hour` |
| `Anthropic-Ratelimit-Unified-5h-Utilization` / `-7d-Utilization` | Anteil 0..1 |
| `Anthropic-Ratelimit-Unified-5h-Reset` / `-7d-Reset` | Unix-Sekunden |
| `Anthropic-Ratelimit-Unified-5h-Status` / `-7d-Status` | wie oben |

Quelle: CLIProxyAPI `internal/runtime/executor/helps/claude_ratelimit.go` liest dieselben Header (Auslastung als Anteil, Reset als Unix-Sekunden oder RFC 3339). `sdk/api/handlers/header_filter.go` reicht sie an tollgate durch.

## meter

- `meter.ParseLimits(h http.Header, now time.Time) (Limits, bool)`: `false`, wenn kein Unified-Header da ist. Kaputte Werte werden zu „unbekannt“ (`nil`), nicht zu 0. Die Auslastung wird auf 0..10 begrenzt, Strings werden auf 64 Bytes gekürzt.
- `meter.LimitTracker` (Mutex): `Observe(h)` übernimmt nur einen Stand mit Headern, `Latest()` liefert eine Kopie.

## server und dashboard

- `server.Options.ResponseObserver func(http.Header)` wird mit den Headern jeder Upstream-Antwort aufgerufen (außer der umgeschriebenen 401), bevor der Body kopiert wird.
- `dashboard.Options.Limits func() (meter.Limits, bool)`. `Status.Limits` ist `null` ohne Messung.
- Zeiten sind RFC 3339 in UTC wie überall in der Dashboard-API.

## Frontend

Accounts zeigt über der Account-Liste den Abschnitt „Abo-Limits“: je Fenster ein Balken mit Prozent, Status als runder Indikator (grün `allowed`, gelb `allowed_warning`, rot `rejected`) und „zurückgesetzt in 2 h 13 min (18:30)“. Ohne Daten steht dort „Noch keine Daten, erscheint nach der nächsten Anfrage“.

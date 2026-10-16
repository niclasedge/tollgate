# Design

## API

Den Vertrag bilden die Go-Typen in `internal/dashboard/types.go`. `web/src/lib/types/dashboard.ts` wird daraus generiert. Alle Routen liegen hinter `auth.RequireAdmin` (Bearer-Token); Client-Keys gelten dort nicht.

- **Traces:** Keyset-Paginierung wie `store.ListTraces` (`before` = `started_at` RFC 3339, `before_id`). `limit` von 1 bis 200, Default 50. `client_name` kommt per Join aus `clients`.
- **Live-Events:** Ein Broker im Prozess verteilt `TraceEvent` an alle SSE-Abonnenten:
  - nach `RecordTrace` mit Status 0, nach `FinishTrace` erneut mit Ergebnis
  - Langsame Abonnenten verlieren Events (gepufferter Kanal, nicht blockierend), der Proxy wartet nie auf das Frontend.
  - Heartbeat-Kommentar alle 15 s.
  - Die Events enthalten dieselben Felder wie die Liste, also nur Metadaten.
- **Kosten:** Eine SQL-Aggregation über abgeschlossene Traces, gruppiert nach UTC-Tag (`started_at` in Millisekunden), Client und Modell. Sie summiert die Usage, die Schätzung und die Ersparnis (`tokens_in` der ersten Stufe minus `tokens_out` der letzten). Die Kosten berechnet `meter.Cost` pro Zeile: Die Preise sind linear, deshalb ergibt die Summe der Usage dieselben Kosten wie die Summe der Einzelkosten.
- **Status:**
  - tollgate: `ok` und die Version aus den Build-Infos.
  - CLIProxyAPI: `GET /healthz` und `GET /v8/management/credentials` auf dem Loopback-Upstream mit dem Management-Key (Header `Authorization: Bearer`), Timeout 3 s.
  - Aus der Antwort übernimmt tollgate nur die Felder von `Account`. Tokens und Dateiinhalte gehen nie durch.

## Frontend

- Das Admin-Token wird beim ersten Aufruf abgefragt und in `localStorage` gehalten, damit es auch auf dem iPhone bleibt. Bei 401 wird es verworfen und erneut abgefragt.
- SSE läuft über `fetch` mit `Authorization`-Header und Stream-Parser statt `EventSource`, damit das Token nie in einer URL steht. `lib/sse.ts` bekommt dafür eine fetch-basierte Quelle.
- **Live:** Liste der neuesten Anfragen. Sie wird per Event aktualisiert, laufende Anfragen tragen einen runden Statusindikator. Ein Klick klappt die Zahlen inline auf.
- **Kosten:** Summen für den Zeitraum (7/30/90 Tage) und Balken pro Tag. Dazu Tabellen pro Client und Modell mit Cache-Trefferquote `cache_read / (input + cache_read + cache_creation)`.
- **Clients:** Liste, Anlegen, Sperren, Rotieren. Der Key erscheint nur einmal, zusammen mit einem kopierfertigen Snippet (`ANTHROPIC_BASE_URL` = Origin der Seite, `ANTHROPIC_AUTH_TOKEN`).
- **Accounts:** aktiver Account mit Status, Health beider Dienste und eine Warnung, wenn mehr als ein Account im auth-dir liegt.

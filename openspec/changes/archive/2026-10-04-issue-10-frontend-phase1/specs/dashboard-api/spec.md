## ADDED Requirements

### Requirement: Dashboard-API hinter dem Admin-Token

tollgate SHALL unter `/api/` die Routen `GET /api/traces`, `GET /api/traces/events`, `GET /api/costs` und `GET /api/status` bereitstellen. Jede Route MUST das Admin-Token verlangen und Client-Keys ablehnen. Antworten MUST NOT Request-Inhalte, Keys oder Tokens enthalten.

#### Scenario: Ohne Admin-Token
- **WHEN** ein Aufruf von `/api/traces` ohne oder mit einem Client-Key erfolgt
- **THEN** antwortet tollgate mit 401, und der Upstream erhält keine Anfrage.

#### Scenario: Traces seitenweise
- **WHEN** der Admin `GET /api/traces?limit=2` und danach mit dem gelieferten Cursor aufruft
- **THEN** erhält er die Traces neueste zuerst, ohne Lücke und ohne Doppelte, mit Client-Name, Usage und Kosten.

### Requirement: Live-Events

`GET /api/traces/events` SHALL ein SSE-Stream sein, der für jeden gestarteten und jeden abgeschlossenen Trace ein Event `trace` sendet. Ein langsamer oder hängender Abonnent MUST NOT den Proxy verzögern.

#### Scenario: Anfrage erscheint live
- **WHEN** ein Admin den Stream abonniert und danach ein Client eine Anfrage stellt
- **THEN** erhält der Admin ein Event mit Status 0 und eines mit dem Endstatus und der Usage.

#### Scenario: Hängender Abonnent
- **WHEN** ein Abonnent keine Daten liest
- **THEN** werden Anfragen weiter ohne Verzögerung beantwortet.

### Requirement: Kostenbericht

`GET /api/costs?days=N` SHALL abgeschlossene Traces der letzten N UTC-Tage (1 bis 366, Default 30) nach Tag, Client und Modell zusammenfassen: Anzahl, Usage, Schätzung, Ersparnis und Kosten nach Listenpreis. Für Modelle ohne Preis MUST `cost_nano_usd` null sein.

#### Scenario: Zwei Modelle an einem Tag
- **WHEN** ein Client an einem Tag je eine Anfrage an `claude-opus-5-5` und an ein unbekanntes Modell stellt
- **THEN** enthält der Bericht zwei Zeilen; die Opus-Zeile trägt Kosten, die andere `null`.

### Requirement: Status und aktiver Account

`GET /api/status` SHALL die Health von tollgate und CLIProxyAPI sowie die Accounts aus dem auth-dir von CLIProxyAPI liefern, gelesen über die Management-API mit dem Key aus `upstream.management_key_file`. Ist der Key nicht konfiguriert oder CLIProxyAPI nicht erreichbar, MUST die Antwort trotzdem 200 sein und den Zustand in `message` beschreiben.

#### Scenario: CLIProxyAPI nicht erreichbar
- **WHEN** der Upstream nicht antwortet
- **THEN** meldet `cliproxyapi.ok` false mit einer Meldung, und `tollgate.ok` bleibt true.

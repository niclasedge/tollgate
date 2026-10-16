## ADDED Requirements

### Requirement: Schlüsselprüfung vor dem Body-Read

Der Server SHALL den Client-Key anhand der Header prüfen, bevor er den Request-Body liest. Eine Anfrage ohne gültigen Key MUST eine 401-Antwort im Anthropic-Fehlerformat erhalten, ohne dass der Body gelesen oder der Upstream aufgerufen wird.

#### Scenario: Unvollständiger Body mit falschem Key
- **WHEN** ein Client `Content-Length: 1000000` ankündigt, nur wenige Bytes sendet und einen unbekannten Key nutzt
- **THEN** antwortet tollgate sofort mit 401, und der Upstream erhält keine Anfrage.

#### Scenario: Gültiger Key
- **WHEN** ein Client mit gültigem Key eine Anfrage sendet
- **THEN** liest tollgate den Body und führt die Pipeline wie bisher aus.

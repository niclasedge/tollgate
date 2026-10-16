## ADDED Requirements

### Requirement: Lokale Routen für Admin-API und Frontend

Der Server SHALL Anfragen auf `/api/clients` und darunter an die Admin-API und `GET`/`HEAD` auf `/`, `/index.html` und `/assets/…` an das eingebettete Frontend geben. Alle übrigen Anfragen MUST unverändert durch den Proxy laufen; der Server MUST NOT Pfade umleiten oder bereinigen.

#### Scenario: Admin-API und Frontend lokal
- **WHEN** ein Client `POST /api/clients` oder `GET /` sendet
- **THEN** beantwortet tollgate die Anfrage selbst, und der Upstream erhält keine Anfrage.

#### Scenario: Ähnliche Pfade gehen an den Proxy
- **WHEN** ein Client `GET /api/clientsX`, `GET /api/other`, `POST /` oder `GET /favicon.ico` sendet
- **THEN** läuft die Anfrage durch die Pipeline zum Upstream.

#### Scenario: Ende-zu-Ende über das Binary
- **WHEN** tollgate startet, ein Admin mit dem Token aus `admin.token_file` einen Client anlegt und der Client mit seinem Key `GET /v1/models` aufruft
- **THEN** erhält der Upstream die Anfrage mit dem tollgate→CPA-Key; nach dem Sperren des Clients wird die nächste Anfrage mit 401 abgelehnt.

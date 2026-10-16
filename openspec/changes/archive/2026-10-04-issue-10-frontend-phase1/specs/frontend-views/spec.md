## ADDED Requirements

### Requirement: Anmeldung mit dem Admin-Token

Das Frontend SHALL beim ersten Aufruf nach dem Admin-Token fragen, es lokal speichern und bei jeder API-Anfrage als Bearer-Token senden, auch für den Live-Stream. Das Token MUST NOT in einer URL erscheinen. Bei 401 MUST es verworfen und neu abgefragt werden.

#### Scenario: Abgelaufenes Token
- **WHEN** die API mit 401 antwortet
- **THEN** zeigt das Frontend wieder die Token-Abfrage.

### Requirement: Bereiche Live, Kosten, Clients und Accounts

Das Frontend SHALL die vier Bereiche mit echten Daten der Dashboard-API und der Client-API zeigen: Live (Anfragen mit Status, Modell, Tokens, Ersparnis, live aktualisiert), Kosten (Tokens und Kosten pro Tag, Client und Modell, Cache-Trefferquote), Clients (erzeugen, sperren, rotieren, Setup-Snippet) und Accounts (aktiver Account, Health beider Dienste).

#### Scenario: Neuer Client
- **WHEN** der Admin einen Client anlegt
- **THEN** zeigt das Frontend den Key genau einmal mit einem kopierfertigen Snippet aus `ANTHROPIC_BASE_URL` und `ANTHROPIC_AUTH_TOKEN`.

#### Scenario: Live-Aktualisierung
- **WHEN** ein `trace`-Event eintrifft
- **THEN** erscheint die Anfrage oben in der Live-Liste oder wird dort aktualisiert, ohne die Seite neu zu laden.

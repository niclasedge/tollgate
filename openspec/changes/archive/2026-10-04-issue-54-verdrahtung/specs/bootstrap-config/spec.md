## ADDED Requirements

### Requirement: Admin-Token-Datei

Die Bootstrap-Konfiguration SHALL `admin.token_file` (Env `TOLLGATE_ADMIN_TOKEN_FILE`) lesen. Fehlt der Wert, MUST der Default `admin.token` im Verzeichnis von `db.path` gelten. Fehlt die Datei beim Start, MUST tollgate sie mit einem zufälligen Token und Modus 0600 anlegen; der Token MUST NOT geloggt werden.

#### Scenario: Erster Start ohne Token-Datei
- **WHEN** tollgate ohne `admin.token_file` und ohne vorhandene Datei startet
- **THEN** liegt danach `admin.token` mit Modus 0600 neben der Datenbank, und die Logzeile nennt nur den Pfad.

#### Scenario: Env überschreibt
- **WHEN** `TOLLGATE_ADMIN_TOKEN_FILE=/a.token` gesetzt ist
- **THEN** verwendet tollgate `/a.token`.

# client-auth Specification

## Purpose
Client-Keys authentifizieren jeden Client gegenüber tollgate; die Admin-API erzeugt, sperrt und rotiert sie, ohne dass ein Key im Klartext gespeichert oder geloggt wird.

## Requirements

### Requirement: Key-Format und Speicherung

Das System SHALL Client-Keys als `tg_` plus 32 Byte aus `crypto/rand` in base64url ohne Padding erzeugen (46 Zeichen). Gespeichert MUST nur der SHA-256 des Keys sowie ein Anzeige-Hinweis (`tg_…` plus letzte vier Zeichen) werden, nie der Klartext.

#### Scenario: Erzeugter Key
- **WHEN** ein Key erzeugt wird
- **THEN** beginnt er mit `tg_`, ist 46 Zeichen lang und unterscheidet sich von jedem früher erzeugten Key.

#### Scenario: Kein Klartext in DB und Logs
- **WHEN** ein Client angelegt, rotiert, gesperrt und genutzt wird
- **THEN** enthalten weder die SQLite-Dateien noch die Log-Ausgabe einen Klartext-Key oder das Admin-Token.

### Requirement: Sofortige Wirkung von Sperren und Rotation

`auth.Verifier` SHALL `pipeline.KeyVerifier` implementieren und die Client-ID für aktive Keys liefern. Unbekannte, fehlerhaft geformte, gesperrte und durch Rotation ersetzte Keys MUST mit `pipeline.ErrInvalidKey` abgewiesen werden, und zwar ab dem nächsten Aufruf. Store-Fehler MUST als interner Fehler (nicht `ErrInvalidKey`) zurückgegeben werden.

#### Scenario: Gesperrter Key
- **WHEN** ein Client gesperrt wird
- **THEN** weist die nächste Prüfung seines Keys ihn ab.

#### Scenario: Rotierter Key
- **WHEN** ein Key rotiert wird
- **THEN** wird der alte Key sofort abgewiesen und der neue akzeptiert.

### Requirement: Admin-API für Client-Keys

Das System SHALL einen `http.Handler` bereitstellen mit `POST /api/clients` (anlegen), `GET /api/clients` (auflisten), `POST /api/clients/{id}/block` (sperren) und `POST /api/clients/{id}/rotate` (rotieren). Der Klartext-Key MUST nur in der Antwort von Anlegen und Rotieren erscheinen; die Liste enthält nur Hinweis, Status und Zeitstempel.

#### Scenario: Anlegen und Auflisten
- **WHEN** ein Client angelegt und danach die Liste abgefragt wird
- **THEN** enthält die Create-Antwort den Key, die Liste jedoch nicht.

#### Scenario: Ungültige Eingaben
- **WHEN** Name leer ist, der Body ungültig ist oder die Client-ID unbekannt ist
- **THEN** antwortet die API mit 400 bzw. 404.

### Requirement: Admin-Token

Die Admin-API SHALL jede Anfrage ohne gültiges `Authorization: Bearer <Admin-Token>` mit 401 abweisen. Der Vergleich MUST in konstanter Zeit erfolgen, Client-Keys MUST nicht als Admin-Token gelten. Das Token SHALL aus einer Datei geladen und bei fehlender Datei mit Modus 0600 neu erzeugt werden; zu kurze Tokens MUST abgelehnt werden. Das Token MUST NOT geloggt werden.

#### Scenario: Falsches oder fehlendes Token
- **WHEN** das Token fehlt, falsch, ein Präfix oder zu lang ist, oder ein Client-Key gesendet wird
- **THEN** antwortet die API mit 401.

#### Scenario: Token-Datei fehlt
- **WHEN** die Token-Datei nicht existiert
- **THEN** wird sie mit Modus 0600 und einem zufälligen Token angelegt und beim nächsten Laden unverändert gelesen.

### Requirement: Interner Speedtest-Client verborgen

Die Admin-API für Client-Keys SHALL den reservierten Client `speedtest` nicht in `GET /api/clients` liefern und MUST Sperren und Rotieren dieser ID mit 404 ablehnen. Anlegen mit einem Namen mit Präfix `tollgate:` MUST mit 400 abgelehnt werden.

#### Scenario: Liste ohne internen Client
- **WHEN** zwei Clients angelegt sind und die Migration den Speedtest-Client angelegt hat
- **THEN** liefert `GET /api/clients` genau zwei Einträge.

#### Scenario: Rotation verweigert
- **WHEN** der Admin `POST /api/clients/speedtest/rotate` aufruft
- **THEN** antwortet tollgate mit 404, und es entsteht kein Key.

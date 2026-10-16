## ADDED Requirements

### Requirement: Lesefrist für den Request-Body nach Stille

tollgate SHALL das Lesen des Request-Body erst abbrechen, wenn der Client für die Dauer der Frist (30 s) keine Daten mehr schickt. Die Gesamtdauer des Uploads MUST NOT begrenzt sein, solange Daten kommen. Ein fehlgeschlagenes Lesen MUST mit Ursache, gelesenen Bytes, angekündigter Länge und Dauer geloggt werden, aber MUST NOT Inhalte des Body enthalten.

#### Scenario: Langsamer Upload über das Relay
- **WHEN** ein Client 2 MB in 45 s gleichmäßig schickt
- **THEN** liest tollgate den ganzen Body und leitet die Anfrage weiter.

#### Scenario: Client verstummt
- **WHEN** ein Client nach der Hälfte des Body 30 s nichts mehr schickt
- **THEN** bricht tollgate ab und loggt `request body read failed` mit `cause=idle_timeout`.

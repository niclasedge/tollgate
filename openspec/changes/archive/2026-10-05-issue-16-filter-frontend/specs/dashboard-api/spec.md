## ADDED Requirements

### Requirement: Trace-Detail mit Stufen

Die Dashboard-API SHALL unter `GET /api/traces/{id}` hinter dem Admin-Token eine Anfrage mit ihrer Zusammenfassung und allen gespeicherten Stufen in Pipeline-Reihenfolge liefern: Name, Dauer, Bytes und Tokens vorher/nachher, beide Hashes, Konfigurationsversion, redigierter Diff und Treffer (Offset, Länge, Typ, Regel, Platzhalter). Eine unbekannte ID MUST mit 404 beantwortet werden. Die Antwort MUST NOT Klartext-Werte enthalten.

#### Scenario: Detail einer redigierten Anfrage
- **WHEN** eine Anfrage mit einer E-Mail-Adresse durch Intake und Redact lief und das Detail abgerufen wird
- **THEN** enthält die Antwort beide Stufen mit Diff und einem Treffer `email` mit Platzhalter `<redacted:email_1>`, aber nicht die E-Mail-Adresse.

#### Scenario: Unbekannte Anfrage
- **WHEN** `GET /api/traces/gibtsnicht` abgerufen wird
- **THEN** antwortet die API mit 404.

### Requirement: Letzte Redactions

Die Dashboard-API SHALL unter `GET /api/redactions?limit=N` (1 bis 50, Standard 10) hinter dem Admin-Token die letzten Treffer aus den Traces liefern, neueste Anfrage zuerst: Zeit, Trace-ID, Client-ID und -Name, Stufe, Typ, Regel und Platzhalter. Die Antwort MUST NOT Werte, Bodies oder Wortlisten-Einträge enthalten.

#### Scenario: Zehn neueste Treffer
- **WHEN** mehrere Anfragen mit zusammen mehr als zehn Treffern gespeichert sind
- **THEN** liefert `/api/redactions` genau zehn Treffer, beginnend mit der neuesten Anfrage.

#### Scenario: Kein Klartext
- **WHEN** eine Anfrage mit einem bekannten Geheimnis durch den Proxy lief
- **THEN** enthält die Antwort von `/api/redactions` den Platzhalter und den Typ, aber das Geheimnis nicht.

#### Scenario: Ungültiges Limit
- **WHEN** `limit=0` oder `limit=51` übergeben wird
- **THEN** antwortet die API mit 400.

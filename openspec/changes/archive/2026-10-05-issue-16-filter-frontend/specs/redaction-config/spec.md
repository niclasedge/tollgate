## ADDED Requirements

### Requirement: Wortliste aus einer Version übernehmen

`POST /api/config/dry-run`, `POST /api/config/import` und `POST /api/config/test` SHALL das Feld `words_from` (Versions-ID, 0 = aus) annehmen. Ist es gesetzt, MUST die Wortliste der gespeicherten Version gelten, und `words.list` im Entwurf MUST leer sein; andernfalls wird mit 422 und einem Issue an `words.list` abgelehnt. Eine unbekannte Version MUST mit 404 abgelehnt werden. Ein Wortlisten-Eintrag, der den Maskierungsfüller `••••••••` enthält, MUST als Validierungsfehler am Pfad des Eintrags abgelehnt werden, ohne den Eintrag zu zitieren.

#### Scenario: Wortliste übernehmen
- **WHEN** Version 2 die Wortliste `Projekt Falke` hat und ein Entwurf mit `words.list: []` und `words_from: 2` importiert wird
- **THEN** hat die neue Version dieselbe Wortliste, und die Antworten enthalten den Eintrag nicht im Klartext.

#### Scenario: Maskierte Fassung importiert
- **WHEN** das YAML aus `GET /api/config/versions/2` unverändert importiert wird
- **THEN** antwortet die API mit 422 und einem Issue an `words.list[0]`.

#### Scenario: Doppelte Wortliste
- **WHEN** ein Entwurf eine Wortliste enthält und `words_from` gesetzt ist
- **THEN** antwortet die API mit 422 und einem Issue an `words.list`.

#### Scenario: UI und YAML gleichwertig
- **WHEN** die über das Formular erzeugte Konfiguration und dasselbe handgeschriebene YAML importiert werden
- **THEN** sind beide Exporte bytegleich.

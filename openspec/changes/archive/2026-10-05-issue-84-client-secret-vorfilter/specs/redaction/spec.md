## ADDED Requirements

### Requirement: client_secret endet am schließenden Anführungszeichen

Folgt der Wert eines `client_secret`-Treffers auf ein öffnendes `"` oder `'`, SHALL die Secret-Spanne am ersten nicht mit `\` maskierten gleichen Anführungszeichen enden; das Anführungszeichen und alles danach MUST stehen bleiben. Ein leerer quotierter Wert MUST NOT als Treffer gelten. Unquotierte Werte und Werte ohne schließendes Anführungszeichen SHALL die Spanne von `\S{8,}` behalten. Der Regex-Text MUST unverändert bleiben.

#### Scenario: JSON-Feld
- **WHEN** der Text `{"client_secret": "S3cretPwd!Long", "id": 1}` oder kompakt `{"client_secret":"S3cretPwd!Long","id":1}` lautet
- **THEN** ist die Ausgabe `{"client_secret": "<redacted:api_key>", "id": 1}` bzw. `{"client_secret":"<redacted:api_key>","id":1}`.

#### Scenario: Maskiertes Anführungszeichen
- **WHEN** der Text `{"client_secret":"abc\"defgh12","x":1}` lautet
- **THEN** ist die Ausgabe `{"client_secret":"<redacted:api_key>","x":1}`.

#### Scenario: Unquotierter Wert
- **WHEN** der Text `ClientSecret=abcdef1234567890` lautet
- **THEN** ist die Ausgabe `ClientSecret=<redacted:api_key>`.

### Requirement: Vorfilter verliert keinen Treffer

Jede eingebaute Regel SHALL eine Liste von Literalen tragen, von denen jeder Treffer ihrer Regex mindestens eines enthält; enthält der Text keines davon, MUST die Regex nicht laufen. Für Regeln mit `(?i)` MUST der Vergleich alle Zeichen berücksichtigen, die Go-`(?i)` auf ein ASCII-Literal faltet (U+017F, U+212A). Eigene Regex und Wortliste SHALL immer laufen. `Find` MUST mit und ohne Vorfilter für jede Eingabe identische Treffer liefern.

#### Scenario: Korpus mit und ohne Vorfilter
- **WHEN** jeder Korpusfall mit Standard- und mit Korpus-Schaltern einmal mit und einmal ohne Vorfilter durch `Find` läuft
- **THEN** sind die Treffer identisch.

#### Scenario: Text ohne Schlüsselwort
- **WHEN** der Text `hello world without anything` lautet
- **THEN** läuft keine eingebaute Regex; bei `PASSWORD is here` läuft nur `generic_assign`.

#### Scenario: Unicode-Faltung
- **WHEN** der Text `paſſword=abcdefgh12` (U+017F) oder `api_Key: abcdefgh12` (U+212A) lautet
- **THEN** findet `Find` mit Vorfilter denselben Treffer wie ohne.

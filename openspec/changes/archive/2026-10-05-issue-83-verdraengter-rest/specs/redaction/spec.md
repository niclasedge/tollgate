## MODIFIED Requirements

### Requirement: Erst sammeln, dann auflösen

Alle aktiven Detektoren SHALL auf dem Originaltext laufen. Überlappende Treffer MUST nach Priorität, dann Spannenlänge, dann Startposition, dann Regelreihenfolge aufgelöst werden, und die Ersetzung MUST in einem Durchgang erfolgen. Ersetzt wird nur die Secret-Gruppe einer Regel; Präfixe wie `password=` oder `Authorization: Bearer ` bleiben stehen. Text in Platzhalterform nach ADR-0006 MUST NOT als Treffer gelten. Wird ein generischer Treffer durch Überlappung verdrängt, MUST jeder nicht abgedeckte Rest seiner Secret-Spanne als eigener Treffer gleicher Regel und gleichen Typs übernommen werden; ein Rest wird an den Rändern um Whitespace gekürzt und entfällt, wenn er keinen Buchstaben und keine Ziffer enthält, ganz im Nicht-Secret-Teil eines anderen Kandidaten liegt, ein Authentifizierungsschema `Bearer`, `Basic` oder `Digest` ist oder in der Allowlist steht.

#### Scenario: Spezifisch vor generisch
- **WHEN** der Text `api_key=sk-ant-…` enthält
- **THEN** stammt der Treffer von `anthropic_key`, und die Ausgabe ist `api_key=<redacted:api_key>`.

#### Scenario: Vorhandener Platzhalter
- **WHEN** der Text `password=<redacted:api_key_1>` enthält
- **THEN** bleibt er unverändert.

#### Scenario: Rest eines verdrängten generischen Treffers
- **WHEN** der Text `Authorization: Bearer <jwt> weiterer-text` lautet
- **THEN** ist die Ausgabe `Authorization: Bearer <redacted:jwt> <redacted:api_key>`, die Treffer stammen von `jwt` und `auth_header`, und `weiterer-text` steht nicht in der Ausgabe.

#### Scenario: Rest zwischen zwei spezifischen Treffern
- **WHEN** der Text `Authorization: a@mailhost.de mitte b@mailhost.de` lautet
- **THEN** ist die Ausgabe `Authorization: <redacted:email> <redacted:api_key> <redacted:email>`.

#### Scenario: Rest ohne Geheimnis bleibt
- **WHEN** der Rest nur aus Satzzeichen besteht (`'Authorization: Bearer <jwt>'`), das Schema vor einem kurzen Token ist (`Authorization: Bearer eyJa.eyJb.c`) oder ein Schlüsselname eines anderen Detektors ist (`Authorization: password=sk-ant-…`)
- **THEN** bleibt er stehen, und nur der spezifische Treffer wird ersetzt.

#### Scenario: Rest in der Allowlist
- **WHEN** die Allowlist `weiterer-text` enthält und der Text `Authorization: Bearer <jwt> weiterer-text` lautet
- **THEN** ist die Ausgabe `Authorization: Bearer <redacted:jwt> weiterer-text`.

## ADDED Requirements
### Requirement: Maske je Treffer
Die Redact-Stufe SHALL für jeden Treffer eine Maske aus Regel, Typ und Klartext berechnen und am Treffer ablegen. Die Maske MUST deterministisch sein, höchstens ein Klarzeichen je geheimem Segment und insgesamt höchstens ein Viertel der Runen des Werts als Klarzeichen zeigen und höchstens 64 Bytes lang sein. Wortliste, Passwörter in Verbindungs-URLs, private Keys und heuristische Key-Treffer MUST ohne Klarzeichen maskiert werden (nur Länge). Der Body an Upstream und der Diff MUST unverändert bleiben.

#### Scenario: E-Mail
- **WHEN** die Adresse `nora.beispiel@firma-beispiel.de` redigiert wird
- **THEN** trägt der Treffer die Maske `n•••@f•••.de`

#### Scenario: API-Key mit bekanntem Präfix
- **WHEN** ein Anthropic-Key der Länge 108 mit Präfix `sk-ant-api03-` redigiert wird
- **THEN** besteht die Maske aus dem Präfix, einem Zeichen, `•••` und der Länge `(108)`

#### Scenario: Wortlisten-Eintrag
- **WHEN** der Wortlisten-Eintrag `Projekt Falke` redigiert wird
- **THEN** lautet die Maske `••• (13)` und enthält kein Zeichen des Eintrags

#### Scenario: Gleicher Wert, gleiche Maske
- **WHEN** derselbe Wert zweimal in einer Anfrage oder in zwei Anfragen redigiert wird
- **THEN** sind beide Masken bytegleich und der Upstream-Body ist derselbe wie ohne Maske

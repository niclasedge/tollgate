## ADDED Requirements
### Requirement: Vorzeichen der Ersparnis in Live
Die Live-Liste SHALL die Ersparnis einer Anfrage als Token-Delta mit genau einem Vorzeichen zeigen: positive Ersparnis mit `−`, negative Ersparnis (Mehrverbrauch) mit `+`, jeweils gefolgt vom deutsch formatierten Betrag. Ohne Ersparnis (0 oder fehlend) SHALL ein Strich `–` erscheinen.
#### Scenario: Positive Ersparnis
- **WHEN** eine Anfrage 1234 Tokens gespart hat
- **THEN** zeigt Live „−1.234“
#### Scenario: Negative Ersparnis
- **WHEN** die Redaction den Text um 12 Tokens verlängert hat (Ersparnis −12)
- **THEN** zeigt Live „+12“ und nie „−-12“
#### Scenario: Keine Ersparnis
- **WHEN** die Ersparnis 0 ist oder fehlt
- **THEN** zeigt Live „–“

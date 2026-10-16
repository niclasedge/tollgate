## ADDED Requirements
### Requirement: Verschlüsselter Tresor und Login
Das Backend SHALL Account-Tokens ausschließlich AES-256-GCM-verschlüsselt speichern und per PKCE mit kurzlebigem, einmaligem State anmelden. Das Frontend SHALL die Login-URL und ein Feld für die Callback-URL anzeigen.
#### Scenario: Zweiter Account
- **WHEN** ein Administrator einen Account hinzufügt
- **THEN** bleibt der bisherige Account aktiv und im auth-dir liegt höchstens eine Datei
#### Scenario: Geheimnisse und Wiederholung
- **WHEN** ein Login fehlschlägt oder ein State wiederverwendet wird
- **THEN** enthalten API und Logs keine Tokens und ein verbrauchter State wird abgelehnt
### Requirement: Manueller Wechsel
Das Backend SHALL laufende Requests schützen, rotierte Tokens vor dem Wechsel sichern, genau eine Datei aktivieren und bei Fehlern den vorherigen Account wiederherstellen.
#### Scenario: Wechsel mit Beispielaufruf
- **WHEN** der Administrator nach dem Cache-Hinweis Aktivieren bestätigt
- **THEN** wird der neue Account aktiviert und ein kurzer Messages-Aufruf ausgeführt; Erfolg oder Fehler werden inline angezeigt
#### Scenario: Upload schlägt fehl
- **WHEN** die neue Datei nicht aktiviert werden kann
- **THEN** wird die vorherige Datei wiederhergestellt und das Ergebnis ohne Tokens angezeigt
#### Scenario: Laufender Stream
- **WHEN** ein Request beim Wechsel noch läuft
- **THEN** wird kein Token entfernt und der Wechsel wird mit verständlicher Meldung abgelehnt

## ADDED Requirements

### Requirement: Stufenbericht im gespeicherten Trace

Der Proxy SHALL Diff und Treffer jeder Stufe in den gespeicherten Trace übernehmen. Mit eingeschalteter Redact-Stufe MUST weder der weitergeleitete Body noch die Store-Datei samt WAL noch die Logs den Klartext eines erkannten Geheimnisses enthalten.

#### Scenario: Leck-Test
- **WHEN** eine Anfrage mit E-Mail-Adresse, API-Key und Verbindungs-Passwort über den Proxy mit Redact-Stufe läuft
- **THEN** enthalten Upstream-Body, DB-Dateien und Log-Ausgabe keinen dieser Werte, und der gespeicherte Trace der Stufe `redact` hat Treffer und einen Diff mit Platzhaltern.

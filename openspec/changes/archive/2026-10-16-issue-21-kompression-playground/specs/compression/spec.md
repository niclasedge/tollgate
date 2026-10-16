## ADDED Requirements

### Requirement: Erklärung der Kompression

tollgate SHALL für einen Befehl und seine Ausgabe erklären, was die Compress-Stufe tut: den normalisierten Befehl, die passende Regel und Strategie, den Text nach jedem Schritt der Strategie und das Ergebnis mit Markerzeile. Ergebnis und Entscheidung MUST mit der Compress-Stufe übereinstimmen. Bleibt die Ausgabe unverändert, MUST der Grund genannt werden. Die Schritte SHALL auch dann gezeigt werden, wenn nur das Größen-Gate greift. Text aus dem Playground MUST NOT gespeichert oder geloggt werden.

#### Scenario: Ausgabe wird komprimiert
- **WHEN** `go test ./...` mit einer langen fehlschlagenden Ausgabe geprüft wird
- **THEN** zeigt die Erklärung Eingabe, die Schritte von `test_log` und das Ergebnis, das Byte für Byte der Ausgabe der Compress-Stufe entspricht.

#### Scenario: Pipe
- **WHEN** `go test ./... | tail` geprüft wird
- **THEN** bleibt die Ausgabe unverändert, und der Grund nennt, dass es kein einzelner Befehl ist.

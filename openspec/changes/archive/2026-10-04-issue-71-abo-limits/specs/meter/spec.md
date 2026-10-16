## ADDED Requirements

### Requirement: Abo-Limits aus Antwort-Headern

`meter.ParseLimits` SHALL aus den Headern `Anthropic-Ratelimit-Unified-Status`, `-Reset`, `-Representative-Claim`, `-5h-Utilization`, `-5h-Reset`, `-5h-Status`, `-7d-Utilization`, `-7d-Reset` und `-7d-Status` einen Stand bilden und `false` liefern, wenn keiner davon vorhanden ist. Ungültige Zahlen und Zeiten MUST als unbekannt gelten, nicht als 0. Resets SHALL als Unix-Sekunden oder RFC 3339 gelesen werden. `meter.LimitTracker` SHALL nebenläufig sicher den letzten Stand mit Zeitpunkt der Messung halten, und eine Antwort ohne diese Header MUST NOT den Stand löschen.

#### Scenario: Vollständige Header
- **WHEN** eine Antwort `5h-Utilization: 0.42`, `5h-Reset: 1790000000` und `5h-Status: allowed` trägt
- **THEN** liefert `ParseLimits` Auslastung 0.42, Reset 1790000000 als Zeit und Status `allowed` für das 5-Stunden-Fenster.

#### Scenario: Kaputte und fehlende Werte
- **WHEN** `7d-Utilization: abc` gesendet wird oder kein Unified-Header vorhanden ist
- **THEN** ist die Wochen-Auslastung unbekannt beziehungsweise `ParseLimits` liefert `false`, und der Tracker behält den vorigen Stand.

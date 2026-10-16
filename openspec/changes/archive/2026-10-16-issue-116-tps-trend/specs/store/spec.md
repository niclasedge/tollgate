## ADDED Requirements

### Requirement: Tokens/s-Urteil speichern

Der Store SHALL das Urteil Tokens pro Sekunde in einer Tabelle `trace_tps` mit Trace-ID, Klasse, Anzahl Vergleichswerte, `tps_milli`, p10, Median, p90, Abweichung in Promille und Urteil speichern. Das Urteil MUST aus der festen Liste `schneller`, `üblich`, `langsamer`, `unbekannt` stammen, Zahlen MUST NOT negativ sein, und die Zeile MUST mit dem Trace gelöscht werden. Die Tabelle MUST NOT Inhalte der Anfrage enthalten. Fehlt eine Zeile, SHALL das Lesen des Traces kein Urteil liefern.

#### Scenario: Schreiben und Lesen
- **WHEN** für einen Trace ein Urteil `langsamer` mit 30 Vergleichswerten gespeichert wird
- **THEN** liefert das Lesen des Traces dieses Urteil mit denselben Zahlen.

#### Scenario: Ungültiges Urteil
- **WHEN** ein Urteil außerhalb der festen Liste gespeichert werden soll
- **THEN** lehnt der Store es mit `ErrInvalid` ab.

#### Scenario: Kaskade
- **WHEN** ein Trace gelöscht wird
- **THEN** ist auch seine Zeile in `trace_tps` gelöscht.

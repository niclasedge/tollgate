## ADDED Requirements

### Requirement: Stufenbericht mit Diff und Treffern

Eine Stufe SHALL über `Request.Report` einen redigierten Diff und Treffer (Offset, Länge, Typ, Regel) melden können. Der Runner MUST den Bericht vor jeder Stufe zurücksetzen und ihn nur bei Erfolg in `StageTrace.Diff` und `StageTrace.Matches` übernehmen. Treffer MUST NOT den ersetzten Wert enthalten.

#### Scenario: Bericht im Trace
- **WHEN** eine Stufe einen Diff und einen Treffer meldet und die nächste Stufe nichts meldet
- **THEN** trägt nur der Trace-Eintrag der ersten Stufe Diff und Treffer.

### Requirement: Platzhalter-Zuordnung im Request

`Request.Placeholders` SHALL die Zuordnung Platzhalter → Klartext der Anfrage für den Rückweg tragen. Das Feld MUST bei JSON-Serialisierung ausgelassen werden und MUST NOT in den Trace gelangen.

#### Scenario: Serialisierung
- **WHEN** ein Request mit gefüllter Zuordnung als JSON serialisiert wird
- **THEN** enthält das Ergebnis keinen Klartext der Zuordnung.

### Requirement: Sitzungs-Pinning über den Store

`SessionPinner` SHALL `ConfigPinner` über den Store umsetzen: Ohne aktive Konfigurationsversion MUST er 0 liefern und nichts schreiben. Mit aktiver Version und Sitzungsschlüssel MUST er die Sitzung pinnen und die zuerst gespeicherte Version liefern. Ohne Sitzungsschlüssel MUST er die aktive Version liefern, ohne zu schreiben.

#### Scenario: Version wechselt mitten in der Sitzung
- **WHEN** eine Sitzung mit aktiver Version 1 beginnt und danach Version 2 aktiviert wird
- **THEN** erhält die Sitzung weiter Version 1, eine neue Sitzung Version 2.

#### Scenario: Keine aktive Version
- **WHEN** im Store keine Version aktiv ist
- **THEN** liefert der Pinner 0, und es entsteht keine Sitzungszeile.

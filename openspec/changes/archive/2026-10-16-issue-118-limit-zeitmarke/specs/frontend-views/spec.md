## ADDED Requirements

### Requirement: Zeitmarke in den Limit-Fenstern

Die Limit-Karte der Übersicht (`LimitCard`) SHALL im Balken des 5-Stunden-Fensters und der Woche, die Abo-Nutzung (`AccountUsage`) im Balken des 5-Stunden-Fensters zusätzlich zur bestehenden Wochenmarke einen Strich für „jetzt“ zeigen. Seine Position SHALL der vergangene Anteil des Fensters sein: (jetzt − (Reset − Fensterlänge)) / Fensterlänge, mit 5 h bzw. 7 Tagen als Fensterlänge. Unter dem Balken SHALL „Strich = jetzt · X % des Fensters vergangen“ (5 Stunden) bzw. „Strich = jetzt · X % der Woche vergangen“ (Woche) stehen. Ohne gültigen Reset, bei abgelaufenem Reset, bei einem Reset weiter als die Fensterlänge entfernt oder ohne Messwert MUST NOT eine Marke erscheinen.

#### Scenario: 5-Stunden-Fenster zu einem Fünftel vergangen
- **WHEN** es 12:00 Uhr ist und das 5-Stunden-Fenster um 16:00 Uhr zurückgesetzt wird
- **THEN** steht der Strich bei 20 % des Balkens und darunter „Strich = jetzt · 20 % des Fensters vergangen“.

#### Scenario: Woche zur Hälfte vergangen in der Übersicht
- **WHEN** die Woche in 3 T 12 h zurückgesetzt wird
- **THEN** zeigt die Limit-Karte im Wochenbalken den Strich bei 50 % und „Strich = jetzt · 50 % der Woche vergangen“.

#### Scenario: Reset unbekannt oder abgelaufen
- **WHEN** der Reset des 5-Stunden-Fensters fehlt, ungültig ist oder in der Vergangenheit liegt
- **THEN** zeigt der 5-Stunden-Balken keinen Strich und keinen Text „Strich = jetzt“.

#### Scenario: Reset weiter entfernt als das Fenster
- **WHEN** der Reset des 5-Stunden-Fensters 6 h entfernt ist
- **THEN** zeigt der 5-Stunden-Balken keinen Strich.

### Requirement: Aktives Abo grün umrandet

Die aktive Abo-Karte SHALL eine dezente Outline in der Statusfarbe „ok“ (`--c-ok`) tragen, in der Limit-Karte der Übersicht und in der Abo-Karte (Übersicht und Accounts), im hellen wie im dunklen Modus. Inaktive Karten MUST NOT diese Outline tragen.

#### Scenario: Aktive Limit-Karte
- **WHEN** die Übersicht die Limit-Karte des aktiven Abos zeigt
- **THEN** trägt die Karte die Markierung „aktiv“ und eine grüne Outline statt des blassen Akzents.

#### Scenario: Inaktives Abo
- **WHEN** ein Abo nicht aktiv ist
- **THEN** trägt seine Karte keine grüne Outline.

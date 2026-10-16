## ADDED Requirements

### Requirement: Platzhalter in Treffern und letzte Treffer

Ein gespeicherter Treffer SHALL zusätzlich seinen Platzhalter enthalten. Der Store MUST nur einen leeren Wert oder einen Platzhalter der Grammatik `<redacted:TYP_N>` (ADR-0006) annehmen und andere Werte mit `ErrInvalid` ablehnen. `RecentRedactions(limit)` SHALL die letzten Treffer über alle Traces liefern, neueste Anfrage zuerst, innerhalb einer Anfrage in Stufen- und Dokumentreihenfolge, mit Zeit, Trace-ID, Client-ID und -Name, Stufe, Typ, Regel und Platzhalter.

#### Scenario: Klartext im Platzhalterfeld
- **WHEN** ein Treffer mit dem Platzhalter `alice@example.com` gespeichert werden soll
- **THEN** lehnt der Store den Trace mit `ErrInvalid` ab.

#### Scenario: Letzte Treffer
- **WHEN** drei Traces mit zusammen zwölf Treffern gespeichert sind und `RecentRedactions(10)` gelesen wird
- **THEN** kommen zehn Treffer, zuerst die der neuesten Anfrage in Dokumentreihenfolge, jeweils mit Platzhalter und Client-Name.

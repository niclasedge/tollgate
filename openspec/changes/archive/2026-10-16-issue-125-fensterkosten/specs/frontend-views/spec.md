## ADDED Requirements

### Requirement: Kosten im Abo-Fenster

Jede Abo-Karte der Übersicht SHALL unter dem 5-h- und dem Wochenfenster die Kosten und die Anzahl der Anfragen dieses Abos im laufenden Fenster zeigen. Gezählt MUST jede Stundenzeile des Abos ab der Stunde, in der das Fenster beginnt (Reset minus Fensterlänge). Fehlt für eine Stunde der Preis, MUST die Summe als Untergrenze („≥“) erscheinen. Ohne laufendes Fenster (Reset unbekannt, vorbei oder weiter als die Fensterlänge entfernt) MUST NOT ein Kostenwert erscheinen. Die Seite Accounts zeigt keine Fensterkosten.

#### Scenario: Kosten im 5-h-Fenster
- **WHEN** das 5-h-Fenster eines Abos um 14:45 UTC zurückgesetzt wird und das Abo um 08:00, 09:00 und 12:00 UTC Kosten hatte
- **THEN** zählt die Karte die Stunden 09:00 und 12:00, nicht 08:00.

### Requirement: Wochenvergleich auf der Übersicht

Die Übersicht SHALL einen Abschnitt „Diese Woche“ zeigen: Kosten, Anfragen, Tokens ein, Tokens aus, Cache-Trefferquote und Gespart der laufenden Kalenderwoche (UTC, ab Montag, bis heute), jeweils mit dem Wert der Vorwoche bis zum selben Wochentag und der Änderung (relativ, bei der Trefferquote in Prozentpunkten). Eine Tabelle SHALL alle sieben Wochentage mit Kosten und Tokens dieser Woche und der Vorwoche zeigen; Tage nach heute MUST leer bleiben. Die Summe der ganzen Vorwoche MUST sichtbar sein.

#### Scenario: Mittwoch
- **WHEN** heute Mittwoch ist und die Vorwoche am Montag $1, am Mittwoch $2 und am Freitag $4 kostete
- **THEN** vergleicht die Kachel „Kosten“ mit $3, die Tabelle zeigt Donnerstag bis Sonntag dieser Woche leer, und die Vorwochensumme beträgt $7.

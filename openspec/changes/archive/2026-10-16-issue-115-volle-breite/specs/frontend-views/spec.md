## ADDED Requirements
### Requirement: Volle Breite des Inhaltsbereichs
Der Inhaltsbereich neben der Sidebar SHALL die gesamte verfügbare Breite nutzen und keine feste Maximalbreite haben. Fließtext-Absätze SHALL für die Lesbarkeit auf etwa 75 Zeichen Zeilenlänge begrenzt bleiben; Tabellen, Listen und Kachel-Grids SHALL nicht begrenzt sein.
#### Scenario: Breiter Bildschirm
- **WHEN** das Frontend bei 1920 px Breite geöffnet ist
- **THEN** reicht der Inhalt jedes Bereichs bis zum rechten Seitenrand (abzüglich Padding)
#### Scenario: Langer Hinweistext
- **WHEN** ein Bereich einen Absatz mit Fließtext zeigt
- **THEN** ist dessen Zeilenlänge auf etwa 75 Zeichen begrenzt

### Requirement: Kachel-Grid der Übersicht
Die Abo-Kacheln der Übersicht SHALL als Grid mit `auto-fit` und `minmax` laufen, sodass bei 1440 px Breite drei Kacheln in einer Zeile stehen und auf schmalen Geräten (390 px) jede Kachel die volle Breite einnimmt und die Kacheln untereinander stehen.
#### Scenario: Drei Abos bei 1440 px
- **WHEN** drei Abos gespeichert sind und das Fenster 1440 px breit ist
- **THEN** stehen alle drei Abo-Kacheln in einer Zeile
#### Scenario: iPhone
- **WHEN** die Übersicht auf einem 390 px breiten Gerät geöffnet ist
- **THEN** stehen die Abo-Kacheln untereinander, ohne horizontal überzulaufen

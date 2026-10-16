## MODIFIED Requirements

### Requirement: Navigation und Bedienung

Die Shell SHALL die Bereiche Übersicht, Live, Anfragen, Kosten & Performance, Filter, Kompression, Clients und Accounts per Hash-Router erreichbar machen, den aktuellen Bereich kennzeichnen und per Tastatur (`0` für Übersicht, `1`–`7` für die übrigen Bereiche) bedienbar sein. Kosten & Performance SHALL unter `#/costs` liegen; `#/performance` MUST dorthin umleiten und den Hash ohne zusätzlichen Verlaufseintrag durch `#/costs` ersetzen. Ein leerer oder unbekannter Hash MUST zur Übersicht führen. Tastenkürzel MUST in Eingabefeldern und mit Modifikatoren ignoriert werden.

#### Scenario: Zifferntaste
- **WHEN** der Nutzer `6` drückt
- **THEN** wechselt die Ansicht zu Clients und der Link trägt `aria-current="page"`.

#### Scenario: Startseite
- **WHEN** die Seite ohne Hash geöffnet wird oder der Nutzer `0` drückt
- **THEN** zeigt sie die Übersicht.

#### Scenario: Alte Performance-Adresse
- **WHEN** die Seite mit `#/performance` geöffnet wird
- **THEN** zeigt sie Kosten & Performance, und der Hash lautet `#/costs`.

### Requirement: Gruppierte, einklappbare Sidebar

Die Sidebar SHALL die Bereiche in den Gruppen Übersicht, Beobachten (Live, Anfragen, Kosten & Performance), Steuern (Filter, Kompression) und Verwalten (Clients, Accounts) mit Gruppenüberschrift, Icon und Label zeigen. Ein eigener Eintrag Performance MUST NOT mehr erscheinen. Mit Admin-Token SHALL sie dezente Zähler für Clients und Accounts zeigen; fehlt ein Wert, MUST der Zähler entfallen statt geraten zu werden. Ein Knopf SHALL sie auf reine Icons einklappen, und der Zustand MUST in `localStorage` erhalten bleiben. Unter 720 px Breite bleibt die Navigation als untere Leiste bedienbar.

#### Scenario: Einklappen
- **WHEN** der Nutzer die Sidebar einklappt und die Seite neu lädt
- **THEN** ist sie weiterhin eingeklappt und der Knopf trägt `aria-expanded="false"`.

#### Scenario: Zähler
- **WHEN** `/api/clients` zwei Clients liefert
- **THEN** zeigt der Eintrag Clients den Zähler 2.

#### Scenario: Zusammengelegter Bereich
- **WHEN** der Nutzer die Taste `3` drückt
- **THEN** öffnet sich Kosten & Performance, und die Gruppe Beobachten enthält genau Live, Anfragen und Kosten & Performance.

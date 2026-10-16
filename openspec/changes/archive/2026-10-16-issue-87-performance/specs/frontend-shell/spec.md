## MODIFIED Requirements

### Requirement: Gruppierte, einklappbare Sidebar

Die Sidebar SHALL die Bereiche in den Gruppen Übersicht, Beobachten (Live, Anfragen, Kosten, Performance), Steuern (Filter, Kompression) und Verwalten (Clients, Accounts) mit Gruppenüberschrift, Icon und Label zeigen. Performance SHALL über die Route `#/performance` und das Tastenkürzel `8` erreichbar sein; die bisherigen Kürzel bleiben unverändert. Mit Admin-Token SHALL sie dezente Zähler für Clients und Accounts zeigen; fehlt ein Wert, MUST der Zähler entfallen statt geraten zu werden. Ein Knopf SHALL sie auf reine Icons einklappen, und der Zustand MUST in `localStorage` erhalten bleiben. Unter 720 px Breite bleibt die Navigation als untere Leiste bedienbar.

#### Scenario: Einklappen
- **WHEN** der Nutzer die Sidebar einklappt und die Seite neu lädt
- **THEN** ist sie weiterhin eingeklappt und der Knopf trägt `aria-expanded="false"`.

#### Scenario: Zähler
- **WHEN** `/api/clients` zwei Clients liefert
- **THEN** zeigt der Eintrag Clients den Zähler 2.

#### Scenario: Performance erreichbar
- **WHEN** der Nutzer die Taste `8` drückt
- **THEN** öffnet sich der Bereich Performance, und er steht in der Gruppe Beobachten nach Kosten.

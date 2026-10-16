# frontend-shell Specification

## Purpose
Das Binary liefert eine navigierbare Frontend-Shell aus, auf der die Fachansichten späterer Issues aufbauen.

## Requirements

### Requirement: Eingebettete Auslieferung

Das Paket `web` SHALL über `Handler()` das gebaute Frontend aus dem Binary ausliefern. Kompilieren und Testen MUST ohne vorherigen Frontend-Build funktionieren; ohne gebautes `dist` SHALL eine Platzhalterseite erscheinen.

#### Scenario: Frischer Checkout
- **WHEN** `go test ./web` ohne `make web` läuft
- **THEN** antwortet `Handler()` auf `/` mit 200 und HTML.

#### Scenario: Gebautes Frontend
- **WHEN** `dist/index.html` existiert
- **THEN** wird dieses ausgeliefert, Dateien unter `/assets/` mit `immutable`-Cache-Header, `index.html` mit `no-cache`.

#### Scenario: Unbekannter Pfad und Methode
- **WHEN** ein unbekannter Pfad angefragt oder ein anderes Verb als GET/HEAD genutzt wird
- **THEN** antwortet der Handler mit 404 bzw. 405.

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

### Requirement: Dark Mode und Responsivität

Die Shell SHALL Hell und Dunkel unterstützen (System-Vorgabe, manuell umschaltbar mit `d` oder Schaltfläche, Wahl gespeichert) und unter 720 px Breite die Navigation als untere Leiste zeigen.

#### Scenario: Umschalten
- **WHEN** der Nutzer `d` drückt
- **THEN** setzt die Seite `data-theme="dark"` und speichert die Wahl.

### Requirement: SSE-Client

Das Frontend SHALL ein SSE-Client-Modul bereitstellen, das benannte Events als JSON dekodiert, den Verbindungsstatus meldet und nach Abbruch mit exponentiellem Backoff neu verbindet, bis es geschlossen wird.

#### Scenario: Abbruch
- **WHEN** die Quelle geschlossen wurde
- **THEN** erfolgt eine Neuverbindung nach 1 s, 2 s, 4 s … bis höchstens 30 s, und nach `close()` keine mehr.

### Requirement: Frontend-Tests in make test

`make test` SHALL Typprüfung und Vitest des Frontends vor den Go-Tests ausführen.

#### Scenario: Fehlschlagender Frontend-Test
- **WHEN** ein Vitest-Test fehlschlägt
- **THEN** endet `make test` mit Fehlerstatus.

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

# Spec Delta

## Purpose

Das Binary liefert eine navigierbare Frontend-Shell aus, auf der die Fachansichten späterer Issues aufbauen.

## ADDED Requirements

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

Die Shell SHALL die Bereiche Live, Anfragen, Kosten, Filter, Kompression, Clients und Accounts per Hash-Router erreichbar machen, den aktuellen Bereich kennzeichnen und per Tastatur (`1`–`7`) bedienbar sein. Tastenkürzel MUST in Eingabefeldern und mit Modifikatoren ignoriert werden.

#### Scenario: Zifferntaste
- **WHEN** der Nutzer `6` drückt
- **THEN** wechselt die Ansicht zu Clients und der Link trägt `aria-current="page"`.

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

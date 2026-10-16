## MODIFIED Requirements

### Requirement: Bereiche Live, Kosten, Clients und Accounts

Das Frontend SHALL die vier Bereiche mit echten Daten der Dashboard-API und der Client-API zeigen: Live (Anfragen mit Status, Modell, Tokens, Ersparnis, live aktualisiert), Kosten & Performance (Dashboard je Account und Details, siehe „Bereich Kosten & Performance“), Clients (erzeugen, sperren, rotieren, Setup-Snippet) und Accounts (aktiver Account, Health beider Dienste).

#### Scenario: Neuer Client
- **WHEN** der Admin einen Client anlegt
- **THEN** zeigt das Frontend den Key genau einmal mit einem kopierfertigen Snippet aus `ANTHROPIC_BASE_URL` und `ANTHROPIC_AUTH_TOKEN`.

#### Scenario: Live-Aktualisierung
- **WHEN** ein `trace`-Event eintrifft
- **THEN** erscheint die Anfrage oben in der Live-Liste oder wird dort aktualisiert, ohne die Seite neu zu laden.

## ADDED Requirements

### Requirement: Bereich Kosten & Performance

Das Frontend SHALL Kosten und Performance auf einer Seite „Kosten & Performance“ mit einem gemeinsamen Zeitraum (Heute, 7 Tage, 30 Tage; Standard 7 Tage) zeigen. Oben SHALL ein Dashboard stehen: die Gesamtkosten groß mit der Anzahl Anfragen und einer Liste je Account mit Kosten, Anteil an den bekannten Kosten in Prozent und Tokens; eine Kostenkurve als gestapeltes Flächendiagramm mit einer Fläche je Account, immer stündlich, mit lückenloser Achse in UTC von der ersten Stunde mit Kosten bis zur aktuellen Stunde, höchstens über die letzten 7 Tage (bei 30 Tagen mit dem Zusatz „letzte 7 Tage“); ohne Kosten MUST statt der Kurve „Keine Kosten im Zeitraum.“ stehen; eine Kennzahlenzeile mit Tokens verarbeitet, Cache gelesen, Eingabe ungecacht, Ausgabe, Cache-Ersparnis in USD und von tollgate gesparten Tokens; das Diagramm „Tokens pro Sekunde · Median je Stunde (UTC)“ mit einer Linie je Modell auf derselben Stundenachse, die Stunden ohne Wert überspringt, mit TTFT-Median und Anteil „langsam“ je Modell. Darunter SHALL der Performance-Detailbereich (Modelltabelle mit Aufklappen, Redactor-Kosten, Speedtest) stehen. Accounts MUST denselben Namen wie im Bereich Accounts tragen; ohne bekannten Namen MUST „Account“ mit den ersten 12 Zeichen der ID und ohne Account „Nicht zugeordnet“ erscheinen. Farben MUST auf das Akzentblau und Grautöne beschränkt sein.

#### Scenario: Anteile je Account
- **WHEN** der Bericht für Account A 3,00 USD und für Account B 1,00 USD liefert und `/api/accounts` A als „Hauptabo“ führt
- **THEN** zeigt das Dashboard Gesamtkosten „$4.00“, „Hauptabo“ mit 75,0 % und den Rückfallnamen von B mit 25,0 %.

#### Scenario: Kostenkurve je Stunde
- **WHEN** für „Heute“ um 04:30 UTC Stunden-Buckets zweier Accounts um 01:00 und 03:00 UTC vorliegen
- **THEN** zeigt die Kurve „Kosten pro Stunde“ zwei Flächen über die Achse 01:00 bis 04:00 UTC, in der 02:00 und 04:00 mit 0 enthalten sind.

#### Scenario: Höchstens 7 Tage
- **WHEN** bei „30 Tage“ Kosten vor 12 Tagen und heute vorliegen
- **THEN** reicht die Achse über die letzten 168 Stunden, und die Kosten von vor 12 Tagen fehlen in der Kurve.

#### Scenario: Kennzahlen
- **WHEN** die Zeilen zusammen 100 Eingabe-, 50 Ausgabe-, 800 Cache-Read- und 10 Cache-Write-Tokens tragen
- **THEN** zeigt die Kennzahlenzeile 960 Tokens verarbeitet, 800 Cache gelesen, 110 Eingabe ungecacht und 50 Ausgabe.

#### Scenario: Alte Performance-Inhalte
- **WHEN** `/api/performance` ein Modell liefert
- **THEN** stehen das Tokens/s-Diagramm im Dashboard und die Modelltabelle mit Einfluss-Matrix, Redactor-Kosten und Speedtest im Detailbereich derselben Seite.

### Requirement: Aufschlüsselung mit Umschalter

Unter dem Dashboard SHALL eine Tabelle „Aufschlüsselung“ mit einem Umschalter Account / Modell / Client / Tag (Standard Account) je Zeile Anfragen, Tokens, Cache-Trefferquote, Cache-Ersparnis, Kosten und Anteil zeigen. Die Ansicht „Tag“ MUST nur Tage mit Anfragen zeigen, neueste zuerst. Unbekannte Kosten MUST als „–“, Teilsummen mit „≥“ erscheinen.

#### Scenario: Umschalten
- **WHEN** der Nutzer „Modell“ wählt
- **THEN** zeigt die Tabelle je Modell eine Zeile, und der Knopf trägt `aria-pressed="true"`.

#### Scenario: Tage ohne Wert
- **WHEN** im Zeitraum von 7 Tagen nur an zwei Tagen Anfragen liegen und der Nutzer „Tag“ wählt
- **THEN** zeigt die Tabelle genau zwei Zeilen.

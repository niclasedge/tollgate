## ADDED Requirements

### Requirement: Tageswerte über den Stundendiagrammen

Die Kostenkurve SHALL über den Stunden jedes UTC-Tags die Kostensumme des Tags zeigen, berechnet aus den Tageszeilen des Kostenberichts; enthält ein Tag Zeilen ohne Preis, MUST die Summe als Untergrenze mit „≥“ erscheinen. Das Tokens/s-Diagramm SHALL über den Stunden jedes UTC-Tags je Modell den Tagesmedian aus `days[].tps_median_milli` zeigen, eine Zeile je Modell in der Farbe seiner Linie; Tage ohne Wert MUST leer bleiben. Mitternacht SHALL durch eine dezente Trennlinie markiert sein. Beschriftungen MUST innerhalb der Zeichenfläche bleiben; würden sie sich überdecken, MUST die ältere entfallen.

#### Scenario: Tagessumme mit unbekanntem Preis
- **WHEN** ein Tag Zeilen über 2,50 USD und ein anderer 4,00 USD plus eine Zeile ohne Preis hat
- **THEN** stehen über der Kurve „$2.50“ und „≥ $4.00“.

#### Scenario: Tagesmedian
- **WHEN** ein Modell an zwei Tagen die Tagesmediane 61,5 und 48,0 t/s hat
- **THEN** stehen über dem Tokens/s-Diagramm „61,5“ und „48,0“.

#### Scenario: Angeschnittener erster Tag
- **WHEN** die 7-Tage-Achse mit der letzten Stunde eines Tags beginnt
- **THEN** entfällt dessen Beschriftung, die übrigen Tage und heute bleiben beschriftet.

### Requirement: Verlauf auf der Übersicht

Die Übersicht SHALL unter den Abo-Limits einen Bereich „Verlauf“ mit der Kostenkurve und dem Tokens/s-Diagramm der letzten 7 Tage samt Tageswerten zeigen und ihn mit der Seite neu laden. Fällt einer der beiden Berichte aus, MUST nur das betroffene Diagramm eine Fehlermeldung zeigen; die übrigen Bereiche bleiben sichtbar.

#### Scenario: Diagramme auf der Übersicht
- **WHEN** die Übersicht geöffnet wird
- **THEN** ruft sie `/api/costs?days=7` und `/api/performance?days=7` ab und zeigt beide Diagramme zwischen „Abo-Limits“ und „Heute“.

#### Scenario: Performance-Bericht fällt aus
- **WHEN** `/api/performance` mit einem Fehler antwortet
- **THEN** zeigt der Verlauf die Kostenkurve und beim Tokens/s-Diagramm die Fehlermeldung.

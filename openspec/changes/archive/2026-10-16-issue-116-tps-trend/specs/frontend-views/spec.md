## ADDED Requirements

### Requirement: Tokens/s in Live

Die Live-Liste SHALL je Anfrage eine Spalte „t/s“ mit den Tokens pro Sekunde ohne Nachkommastelle und darunter der Abweichung zum Median in Prozent zeigen. `langsamer` MUST in der Fehlerfarbe, `schneller` im Akzentblau und `üblich` grau erscheinen; bei `unbekannt` steht nur die Zahl. Ein Tooltip SHALL Tokens/s, Median, p10 bis p90, Anzahl Vergleichswerte und Klasse nennen. Laufende Anfragen und Antworten ohne Tokens/s MUST „–“ zeigen. Das Inline-Detail SHALL dieselbe Aussage als Zeile „Tokens/s“ zeigen. Der rote Statuspunkt MUST NOT vom Tokens/s-Urteil abhängen.

#### Scenario: Langsamere Generierung
- **WHEN** ein Trace `tps.verdict = langsamer`, 48,2 Tokens/s, Median 62,0 und −225 ‰ meldet
- **THEN** zeigt die Spalte „48“ und darunter „−23 %“ in der Fehlerfarbe.

#### Scenario: Laufende Anfrage
- **WHEN** ein Trace noch läuft
- **THEN** zeigt die Spalte „–“.

### Requirement: Einfluss-Matrix und Trend

Die Performance-Ansicht SHALL je Modell aufklappbar eine Tabelle Eingabe-Klasse × Cache-Klasse mit Median TTFT, Median Tokens/s und Anzahl je Zelle zeigen, den Zellhintergrund in einem einzigen Blauton nach Median TTFT, die Kernaussagen als Sätze (z. B. „Warmer Cache (≥ 90 %) senkt TTFT bei 32k–128k Eingabe im Median um 41 %.“) und den Hinweis „Zusammenhang aus den eigenen Daten, keine Ursache.“ Der Tagesverlauf SHALL Median TTFT, Median Tokens/s und Median Cache-Anteil als schmale Linien zeigen. Zellen ohne Median MUST nur die Anzahl zeigen. Über der Modelltabelle SHALL ein Diagramm „Tokens pro Sekunde · Median je Stunde (UTC)“ mit einer Linie je Modell, einer Stundenachse von der ersten Stunde mit Werten bis jetzt (höchstens 7 Tage) und einer Legende mit dem letzten Median je Modell stehen; Stunden ohne Werte MUST übersprungen werden.

#### Scenario: Kernaussage
- **WHEN** die API für `i2` eine Kernaussage `cache_ttft` mit −411 ‰ liefert
- **THEN** zeigt die Ansicht „Warmer Cache (≥ 90 %) senkt TTFT bei 32k–128k Eingabe im Median um 41 %.“

#### Scenario: Tokens/s-Verlauf aller Modelle
- **WHEN** der Bericht für 7 Tage zwei Modelle mit Tages-Medianen liefert, eines davon nur für den letzten Tag
- **THEN** zeigt das Diagramm zwei Linien über alle 7 Tage, die zweite nur als Punkt am letzten Tag, und die Legende nennt beide Modelle mit ihrem letzten Median in t/s.

#### Scenario: Dünne Zelle
- **WHEN** eine Zelle 3 Werte und keine Mediane hat
- **THEN** zeigt sie nur „n = 3“ ohne Hintergrundfarbe.

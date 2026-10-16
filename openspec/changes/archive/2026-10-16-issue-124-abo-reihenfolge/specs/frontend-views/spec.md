## ADDED Requirements

### Requirement: Abo-Karten nach Verfügbarkeit

Die Übersicht SHALL die Abo-Karten in dieser Reihenfolge zeigen: das aktive Abo; Abos mit Wochenkontingent und freiem 5-h-Fenster, nach nächstem 5-h-Reset (ohne laufendes Fenster zuerst); Abos mit Wochenkontingent und vollem 5-h-Fenster, nach nächstem 5-h-Reset; Abos mit verbrauchtem Wochenkontingent, nach Wochen-Reset; Abos ohne Nutzungsdaten. Ein Reset in der Vergangenheit MUST als neues, freies Fenster zählen. Bei Gleichstand MUST die Reihenfolge nach E-Mail (sonst Name) eindeutig sein.

#### Scenario: Fenster des aktiven Abos voll
- **WHEN** das aktive Abo sein 5-h-Limit erreicht hat, ein Abo mit vollem 5-h-Fenster in 1 h zurückgesetzt wird und ein anderes mit 60 % Auslastung frei ist
- **THEN** steht nach dem aktiven Abo zuerst das freie, danach das volle.

#### Scenario: Wochenkontingent verbraucht
- **WHEN** ein Abo 100 % der Woche verbraucht hat
- **THEN** steht es hinter allen Abos mit Wochenkontingent.

### Requirement: Kompakte Abo-Karten

Eine Abo-Karte SHALL die E-Mail als Titel zeigen und den hinterlegten Namen als Tooltip; ohne E-Mail MUST der Name der Titel sein. Der Reset eines Fensters SHALL in derselben Zeile rechts neben der Prozentzahl stehen. Eine Textzeile zur Erklärung der Zeitmarke MUST NOT erscheinen; die Marke MUST ihr Label und ihren Tooltip behalten.

#### Scenario: Karte mit E-Mail
- **WHEN** ein Abo den Namen „Hauptabo“ und die E-Mail `a@example.com` hat
- **THEN** lautet der Titel `a@example.com`, der Name erscheint nur als Tooltip, und die E-Mail steht nicht ein zweites Mal auf der Karte.

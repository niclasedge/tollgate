## ADDED Requirements

### Requirement: Redact-Stufe mit nummerierten Platzhaltern

Die Stufe `redact` SHALL bei `POST /v1/messages` und `POST /v1/messages/count_tokens` jedes Geheimnis in den Zielen von `jsonsplice.Scan` durch einen Platzhalter `<redacted:TYP_N>` nach der Grammatik aus ADR-0006 ersetzen. Die Nummer N MUST pro Anfrage und pro Typ in Dokumentreihenfolge beim ersten Auftreten eines Werts vergeben werden; derselbe Wert MUST innerhalb der Anfrage dieselbe Nummer erhalten. Nummern, die bereits als Platzhaltertext in einem Ziel stehen, MUST übersprungen werden. Andere Endpunkte MUST unverändert bleiben. Passt eine Nummer nicht in die Grammatik, MUST die Stufe die Anfrage abbrechen.

#### Scenario: Nummerierung in Dokumentreihenfolge
- **WHEN** die Nachrichten nacheinander `a@mailhost.de`, `b@mailhost.de` und wieder `a@mailhost.de` enthalten
- **THEN** werden daraus `<redacted:email_1>`, `<redacted:email_2>` und `<redacted:email_1>`.

#### Scenario: Reservierte Nummer
- **WHEN** ein User-Text bereits `<redacted:email_1>` enthält und später eine E-Mail-Adresse folgt
- **THEN** wird die Adresse zu `<redacted:email_2>`, und `<redacted:email_1>` steht nicht in der Zuordnung.

#### Scenario: count_tokens
- **WHEN** dieselbe Anfrage an `/v1/messages/count_tokens` geht
- **THEN** ist der weitergeleitete Body identisch mit dem von `/v1/messages`.

#### Scenario: Durchgereichter Endpunkt
- **WHEN** eine Anfrage an `/v1/models` geht
- **THEN** liefert die Stufe keine Ersetzungen.

### Requirement: Nur erlaubte Blöcke

Die Stufe SHALL nur User-Strings, `text`-Blöcke von User und Assistant, `tool_result`-Inhalte und alle String-Werte in `tool_use.input` redigieren. `thinking`, `redacted_thinking`, Signaturen, `system`, `tools`, `metadata`, Bilder, Dokumente, unbekannte Blocktypen und unbekannte Felder MUST Byte für Byte erhalten bleiben, auch wenn sie Geheimnisse enthalten. Platzhalter in Thinking-Blöcken MUST NOT Nummern reservieren.

#### Scenario: Thinking mit Geheimnis
- **WHEN** ein Assistant-Turn einen `thinking`-Block mit E-Mail-Adresse und Signatur sowie einen `text`-Block mit derselben Adresse enthält
- **THEN** ist nur der `text`-Block verändert, und Thinking samt Signatur stehen bytegleich im Ergebnis.

#### Scenario: Unbekannter Blocktyp
- **WHEN** ein Block vom Typ `future_block` ein Geheimnis und unbekannte Felder enthält
- **THEN** bleibt der gesamte Body außerhalb der erlaubten Ziele bytegleich.

### Requirement: Zuordnung nur im RAM der Anfrage

Die Zuordnung Platzhalter → Klartext SHALL nur im RAM der Anfrage liegen (`Request.Placeholders`) und mit ihr enden. Sie MUST NOT in Trace, Diff, Treffern, Logs, DB oder Frontend erscheinen. Wird jeder Platzhalter der Zuordnung im Ergebnis zurückübersetzt, MUST jedes Ziel exakt seinem Eingabewert entsprechen.

#### Scenario: Rehydrate ergibt das Original
- **WHEN** beliebiger Text als User-Nachricht redigiert und danach mit der Zuordnung zurückübersetzt wird
- **THEN** stimmt jedes Ziel Byte für Byte mit der Eingabe überein.

### Requirement: Deterministisch und cachefest

Gleiche Anfrage und gleiche Konfiguration SHALL Byte für Byte denselben Body, dieselbe Zuordnung, denselben Diff und dieselben Treffer ergeben. Wächst der Verlauf nur nach hinten, MUST der redigierte Body des früheren Turns ein Byte-Präfix des redigierten Bodys des späteren Turns bis zum Ende der früheren Nachrichten sein.

#### Scenario: Drei Turns
- **WHEN** drei aufeinanderfolgende Anfragen einer Sitzung denselben Verlauf um je einen Turn mit neuen und wiederholten Geheimnissen verlängern
- **THEN** sind die redigierten Nachrichten der früheren Turns in jeder späteren Anfrage bytegleich, und wiederholte Geheimnisse tragen dieselbe Nummer.

### Requirement: Eingebaute Konfiguration builtin-v1

Bis zur versionierten Konfiguration (#15) SHALL die Stufe über einen `Resolver` die eingebaute Konfiguration `builtin-v1` (`DefaultOptions()`) für jede gepinnte Version verwenden. Der Resolver MUST die Version aus `Request.ConfigVersion` erhalten.

#### Scenario: Version wird übergeben
- **WHEN** der Request auf Version 7 gepinnt ist
- **THEN** ruft die Stufe den Resolver mit Version 7 auf.

### Requirement: Hinweis auf den cc_version-Fingerprint

Ändert die Stufe in einem Text der ersten User-Nachricht eines der UTF-16-Zeichen an Index 4, 7 oder 20, SHALL die erste Diff-Zeile `note: cc_version fingerprint input changed` lauten. Der `system`-Block MUST unverändert bleiben.

#### Scenario: Geheimnis am Anfang
- **WHEN** die erste User-Nachricht mit einer E-Mail-Adresse beginnt
- **THEN** beginnt der Diff mit dem Hinweis; steht die Adresse erst nach Zeichen 30, fehlt er.

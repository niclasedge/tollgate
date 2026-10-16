# redaction Specification

## Purpose
Erkennung von Geheimnissen im Text: eingebaute Detektoren aus dem Go-Port des hook-redactor (ADR-0005), Auflösung überlappender Treffer in einem Durchgang und Parität zur Rust-Version über einen gemeinsamen Korpus.

## Requirements

### Requirement: Eingebaute Detektoren nach ADR-0005

`internal/redact` SHALL die 19 in ADR-0005 zugeordneten Rust-Regeln mit unveränderten Regex-Texten, den False-Positive-Filtern (private IPv4 und IPv6, Versions-Kontext, E-Mail-Ausnahmen, exakte Whitelist) und den sechs Schaltern `email`, `api_keys`, `jwt`, `connection_urls`, `ip_addresses`, `private_keys` bereitstellen. `ip_addresses` MUST standardmäßig aus sein, alle anderen an. `connection_urls` MUST nur das Passwort in der Benutzerinfo einer Verbindungs-URL treffen. Die nicht portierten Regeln aus ADR-0005 MUST NOT enthalten sein.

#### Scenario: Schalter aus
- **WHEN** `ip_addresses` aus ist und der Text `DNS 8.8.8.8` enthält
- **THEN** bleibt die Adresse stehen; mit `ip_addresses: true` wird sie ersetzt.

#### Scenario: Verbindungs-URL
- **WHEN** der Text `postgres://app:s3cretpw@db.prod.io:5432/x` enthält
- **THEN** lautet die Ausgabe `postgres://app:<redacted:connection_password>@db.prod.io:5432/x`, und eine URL ohne Passwort bleibt unverändert.

#### Scenario: False Positive
- **WHEN** der Text `Version 1.2.3.4 released` oder `noreply@example.org` enthält
- **THEN** wird nichts ersetzt.

### Requirement: Erst sammeln, dann auflösen

Alle aktiven Detektoren SHALL auf dem Originaltext laufen. Überlappende Treffer MUST nach Priorität, dann Spannenlänge, dann Startposition, dann Regelreihenfolge aufgelöst werden, und die Ersetzung MUST in einem Durchgang erfolgen. Ersetzt wird nur die Secret-Gruppe einer Regel; Präfixe wie `password=` oder `Authorization: Bearer ` bleiben stehen. Text in Platzhalterform nach ADR-0006 MUST NOT als Treffer gelten. Wird ein generischer Treffer durch Überlappung verdrängt, MUST jeder nicht abgedeckte Rest seiner Secret-Spanne als eigener Treffer gleicher Regel und gleichen Typs übernommen werden; ein Rest wird an den Rändern um Whitespace gekürzt und entfällt, wenn er keinen Buchstaben und keine Ziffer enthält, ganz im Nicht-Secret-Teil eines anderen Kandidaten liegt, ein Authentifizierungsschema `Bearer`, `Basic` oder `Digest` ist oder in der Allowlist steht.

#### Scenario: Spezifisch vor generisch
- **WHEN** der Text `api_key=sk-ant-…` enthält
- **THEN** stammt der Treffer von `anthropic_key`, und die Ausgabe ist `api_key=<redacted:api_key>`.

#### Scenario: Vorhandener Platzhalter
- **WHEN** der Text `password=<redacted:api_key_1>` enthält
- **THEN** bleibt er unverändert.

#### Scenario: Rest eines verdrängten generischen Treffers
- **WHEN** der Text `Authorization: Bearer <jwt> weiterer-text` lautet
- **THEN** ist die Ausgabe `Authorization: Bearer <redacted:jwt> <redacted:api_key>`, die Treffer stammen von `jwt` und `auth_header`, und `weiterer-text` steht nicht in der Ausgabe.

#### Scenario: Rest zwischen zwei spezifischen Treffern
- **WHEN** der Text `Authorization: a@mailhost.de mitte b@mailhost.de` lautet
- **THEN** ist die Ausgabe `Authorization: <redacted:email> <redacted:api_key> <redacted:email>`.

#### Scenario: Rest ohne Geheimnis bleibt
- **WHEN** der Rest nur aus Satzzeichen besteht (`'Authorization: Bearer <jwt>'`), das Schema vor einem kurzen Token ist (`Authorization: Bearer eyJa.eyJb.c`) oder ein Schlüsselname eines anderen Detektors ist (`Authorization: password=sk-ant-…`)
- **THEN** bleibt er stehen, und nur der spezifische Treffer wird ersetzt.

#### Scenario: Rest in der Allowlist
- **WHEN** die Allowlist `weiterer-text` enthält und der Text `Authorization: Bearer <jwt> weiterer-text` lautet
- **THEN** ist die Ausgabe `Authorization: Bearer <redacted:jwt> weiterer-text`.

### Requirement: Determinismus und Robustheit

Gleiche Eingabe und gleiche Optionen SHALL Byte für Byte die gleiche Ausgabe und die gleichen Treffer liefern. Beliebige Eingaben, auch ungültiges UTF-8, MUST ohne Panic verarbeitet werden, und der Klartext eines übernommenen Treffers MUST NOT in der Ausgabe stehen, sofern er nicht an anderer, nicht erkannter Stelle der Eingabe vorkommt.

#### Scenario: Wiederholter Aufruf
- **WHEN** derselbe Text zweimal redigiert wird
- **THEN** sind Ausgabe und Treffer identisch.

### Requirement: Korpus und Paritätstest

Die relevanten Testfälle aus `src/patterns.rs`, `src/redactor.rs`, `tests/red_team.rs` (einschließlich §7 und §9), `tests/eu_pii_patterns.rs` und `tests/integration.rs` SHALL als txtar-Korpus unter `internal/redact/testdata/` vorliegen und gegen den Go-Port laufen. `parity_test.go` SHALL denselben Korpus durch das Rust-Binary aus `TOLLGATE_REDACTOR_REPO` (Standard `ref/hook-redactor`) schicken und normalisiert vergleichen; ohne Repo oder Binary MUST der Test übersprungen werden. Jede Abweichung, die nicht als bewusste Abweichung aus ADR-0005 benannt und im Fall markiert ist, MUST den Test fehlschlagen lassen, ebenso eine markierte Abweichung, die nicht mehr auftritt.

#### Scenario: Rust-Repo vorhanden
- **WHEN** `ref/hook-redactor/target/release/redactor` existiert
- **THEN** läuft jeder Korpusfall durch beide Implementierungen, und nur markierte Fälle unterscheiden sich.

#### Scenario: Rust-Repo fehlt
- **WHEN** `TOLLGATE_REDACTOR_REPO` auf ein Verzeichnis ohne Binary zeigt
- **THEN** wird der Paritätstest übersprungen.

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

### Requirement: Rückübersetzung bekannter Platzhalter

Der Rückweg SHALL jeden Platzhalter, der in der Zuordnung der Anfrage (`Request.Placeholders`) steht, durch seinen Klartext ersetzen. Platzhalter ohne Eintrag in der Zuordnung MUST unverändert bleiben. In `input_json_delta` und JSON-Kontexten MUST der Klartext nach ADR-0004 JSON-escaped eingesetzt werden. Bei leerer Zuordnung MUST die Antwort Byte für Byte unverändert bleiben.

#### Scenario: Bekannter und unbekannter Platzhalter
- **WHEN** ein Text `<redacted:email_1>` und `<redacted:email_9>` enthält und nur `email_1` in der Zuordnung steht
- **THEN** wird nur `<redacted:email_1>` ersetzt.

#### Scenario: Escaping in input_json_delta
- **WHEN** der Klartext Anführungszeichen, Backslash oder Zeilenumbruch enthält und der Platzhalter in `partial_json` steht
- **THEN** ergibt das zusammengesetzte `partial_json` gültiges JSON, dessen String-Wert genau den Klartext enthält.

### Requirement: Puffer pro Content-Block im Stream

Der Rückweg SHALL im SSE-Stream pro Content-Block (`index`) einen eigenen Puffer führen. Endet ein `text_delta` oder `input_json_delta` mit einem Präfix, der noch zu einem Platzhalter werden kann, MUST dieser Rest (höchstens 49 Bytes) zurückgehalten und dem nächsten Delta desselben Blocks vorangestellt werden. Ein Delta, das dadurch leer wird, MUST trotzdem mit leerem Text weitergeleitet werden. Bei `content_block_stop` MUST ein verbleibender Rest ungeprüft als eigenes Delta vor dem Stop-Event ausgegeben werden. Die Ausgabe MUST unabhängig davon sein, wie der Upstream die Bytes auf Chunks verteilt.

#### Scenario: Platzhalter über Delta-Grenzen
- **WHEN** `<redacted:email_1>` auf drei aufeinanderfolgende `text_delta` verteilt ist
- **THEN** enthält der zusammengesetzte Text des Blocks den Klartext, und die Zahl der Delta-Events bleibt gleich.

#### Scenario: Verschränkte Blöcke
- **WHEN** Deltas der Blöcke 0 und 1 abwechselnd kommen und jeder einen Teil eines Platzhalters trägt
- **THEN** wird jeder Block nur mit seinen eigenen Deltas zusammengesetzt.

#### Scenario: Unvollständiger Rest am Blockende
- **WHEN** ein Block mit `<redac` endet
- **THEN** steht `<redac` unverändert in einem Delta vor `content_block_stop`.

#### Scenario: Chunk-Grenzen im HTTP-Stream
- **WHEN** ein Mock-Upstream (`httptest`) denselben Stream byteweise, in zufälligen und in ganzen Stücken mit Flush sendet
- **THEN** ist die Ausgabe in allen Fällen Byte für Byte gleich.

### Requirement: Thinking und unbekannte Events unverändert

`thinking_delta`, `signature_delta`, `content_block_start`, `message_start`, `message_delta`, `message_stop`, `ping`, `error`, Kommentare und unbekannte Events oder Delta-Typen SHALL Byte für Byte durchgereicht werden. In umgeschriebenen Deltas MUST sich nur der Wert von `delta.text` bzw. `delta.partial_json` ändern; alle anderen Bytes des Events bleiben erhalten.

#### Scenario: Thinking mit Platzhalter
- **WHEN** ein `thinking_delta` `<redacted:email_1>` enthält
- **THEN** wird das Event unverändert weitergeleitet.

#### Scenario: Unbekannte Felder
- **WHEN** ein `text_delta`-Event zusätzliche, unbekannte Felder trägt
- **THEN** stehen diese Felder bytegleich im umgeschriebenen Event.

### Requirement: Nicht-Streaming-Antworten

Bei einer JSON-Antwort SHALL der Rückweg nur `content[].text` von `text`-Blöcken und alle String-Werte in `content[].input` von `tool_use`-Blöcken per Byte-Splicing zurückübersetzen. Thinking-Blöcke, Signaturen und alle übrigen Felder MUST bytegleich bleiben. Ist der Body kein gültiges JSON-Objekt, MUST er unverändert bleiben.

#### Scenario: text und tool_use
- **WHEN** eine Antwort einen `thinking`-Block, einen `text`-Block und einen `tool_use`-Block mit `<redacted:email_1>` enthält
- **THEN** sind `text` und `tool_use.input` zurückübersetzt, der Thinking-Block ist bytegleich.

### Requirement: Redact und Rehydrate sind invers

Wird ein beliebiger Text redigiert und das Ergebnis über den Stream-Rückweg in beliebigen Delta-Stücken zurückübersetzt, SHALL das Ergebnis exakt dem Ausgangstext entsprechen, auch wenn der Ausgangstext selbst Platzhaltertext enthält (reservierte Nummern stehen nicht in der Zuordnung).

#### Scenario: Fuzz
- **WHEN** der Fuzz-Test beliebige Texte redigiert und den redigierten Text an beliebigen Stellen in Deltas teilt
- **THEN** ergibt der zurückübersetzte Blocktext den Ausgangstext, für `text_delta` und `input_json_delta`.

### Requirement: Eigene Regex-Filter

`redact.Compile` SHALL eigene Regeln aus `{name, pattern, placeholder, examples}` mit Priorität 100 über den eingebauten Regeln einbauen. Eine Konfiguration MUST abgelehnt werden, wenn ein Muster nicht kompiliert, den leeren Text trifft, ein Beispiel nicht trifft, ein Name doppelt ist oder der Platzhalter-Typ nicht der Grammatik `[a-z][a-z0-9_]{0,31}` aus ADR-0006 entspricht oder einem eingebauten Typ bzw. `word` gleicht.

#### Scenario: Eigene Kennung
- **WHEN** die Regel `kundennummer` mit Muster `KD-\d{6}` und Platzhalter `customer_id` aktiv ist und der Text `Ticket KD-123456` lautet
- **THEN** lautet die Ausgabe `Ticket <redacted:customer_id>`, Regel `custom:kundennummer`.

#### Scenario: Beispiel trifft nicht
- **WHEN** die Regel als Beispiel `KD-12` angibt
- **THEN** schlägt `Compile` mit dem Pfad `custom[0].examples[0]` fehl.

### Requirement: Wortliste

`redact.Compile` SHALL jeden Eintrag der Wortliste mit Priorität 110 und Typ `word` erkennen. `case_sensitive: false` MUST Groß- und Kleinschreibung Unicode-bewusst ignorieren. `whole_word: true` MUST Treffer verwerfen, deren Nachbarzeichen Buchstabe, Ziffer oder Kombinationszeichen sind, sofern der Rand des Eintrags selbst ein solches Zeichen ist. Mehrwort-Einträge MUST mit beliebigem Whitespace einschließlich Zeilenumbruch treffen. Fehlermeldungen MUST NOT Einträge der Wortliste zitieren. Die Reihenfolge MUST unabhängig von Map-Iteration sein.

#### Scenario: Ganze Wörter mit Umlaut
- **WHEN** die Wortliste `Müller` mit `whole_word: true` enthält und der Text `Müllers Akte, Herr Müller.` lautet
- **THEN** wird nur das zweite Vorkommen ersetzt; mit `whole_word: false` beide.

#### Scenario: Groß- und Kleinschreibung
- **WHEN** `case_sensitive: false` gilt und der Text `MUSTER GMBH` enthält
- **THEN** wird `Muster GmbH` erkannt; mit `case_sensitive: true` nicht.

#### Scenario: Mehrwort über Zeilenumbruch
- **WHEN** die Wortliste `Projekt Falke` enthält und der Text `Projekt\nFalke` lautet
- **THEN** wird die ganze Spanne ersetzt.

### Requirement: Allowlist

Ein Wert der Allowlist SHALL nie ersetzt werden, gleich welcher Detektor ihn findet. Der Vergleich MUST exakt auf dem Wert erfolgen, der ersetzt würde. Die eingebauten E-Mail-Ausnahmen MUST zusätzlich aktiv bleiben.

#### Scenario: Allowlist hat Vorrang
- **WHEN** die Allowlist `team@muster-gmbh.invalid` enthält und der Text diese und `jane@muster-gmbh.invalid` enthält
- **THEN** bleibt die erste Adresse stehen und die zweite wird ersetzt.

### Requirement: client_secret endet am schließenden Anführungszeichen

Folgt der Wert eines `client_secret`-Treffers auf ein öffnendes `"` oder `'`, SHALL die Secret-Spanne am ersten nicht mit `\` maskierten gleichen Anführungszeichen enden; das Anführungszeichen und alles danach MUST stehen bleiben. Ein leerer quotierter Wert MUST NOT als Treffer gelten. Unquotierte Werte und Werte ohne schließendes Anführungszeichen SHALL die Spanne von `\S{8,}` behalten. Der Regex-Text MUST unverändert bleiben.

#### Scenario: JSON-Feld
- **WHEN** der Text `{"client_secret": "S3cretPwd!Long", "id": 1}` oder kompakt `{"client_secret":"S3cretPwd!Long","id":1}` lautet
- **THEN** ist die Ausgabe `{"client_secret": "<redacted:api_key>", "id": 1}` bzw. `{"client_secret":"<redacted:api_key>","id":1}`.

#### Scenario: Maskiertes Anführungszeichen
- **WHEN** der Text `{"client_secret":"abc\"defgh12","x":1}` lautet
- **THEN** ist die Ausgabe `{"client_secret":"<redacted:api_key>","x":1}`.

#### Scenario: Unquotierter Wert
- **WHEN** der Text `ClientSecret=abcdef1234567890` lautet
- **THEN** ist die Ausgabe `ClientSecret=<redacted:api_key>`.

### Requirement: Vorfilter verliert keinen Treffer

Jede eingebaute Regel SHALL eine Liste von Literalen tragen, von denen jeder Treffer ihrer Regex mindestens eines enthält; enthält der Text keines davon, MUST die Regex nicht laufen. Für Regeln mit `(?i)` MUST der Vergleich alle Zeichen berücksichtigen, die Go-`(?i)` auf ein ASCII-Literal faltet (U+017F, U+212A). Eigene Regex und Wortliste SHALL immer laufen. `Find` MUST mit und ohne Vorfilter für jede Eingabe identische Treffer liefern.

#### Scenario: Korpus mit und ohne Vorfilter
- **WHEN** jeder Korpusfall mit Standard- und mit Korpus-Schaltern einmal mit und einmal ohne Vorfilter durch `Find` läuft
- **THEN** sind die Treffer identisch.

#### Scenario: Text ohne Schlüsselwort
- **WHEN** der Text `hello world without anything` lautet
- **THEN** läuft keine eingebaute Regex; bei `PASSWORD is here` läuft nur `generic_assign`.

#### Scenario: Unicode-Faltung
- **WHEN** der Text `paſſword=abcdefgh12` (U+017F) oder `api_Key: abcdefgh12` (U+212A) lautet
- **THEN** findet `Find` mit Vorfilter denselben Treffer wie ohne.

### Requirement: Kunden-, Vornamen- und Nachnamenlisten

`redact.Compile` SHALL neben der Wortliste drei weitere Listen annehmen: Kunden mit dem Platzhalter-Typ `kunde`, Vornamen mit `vorname` und Nachnamen mit `nachname`. Sie MUST dieselbe Treffersemantik wie die Wortliste haben (`case_sensitive`, `whole_word`, Unicode-Wortgrenzen, mehrteilige Einträge, längster Treffer zuerst, Allowlist, Vorrang vor eigenen Regeln und eingebauten Filtern).

#### Scenario: Kunde, Vorname und Nachname
- **WHEN** Kunde `Muster GmbH`, Vorname `Anna` und Nachname `Müller` konfiguriert sind und der Text `Anna Müller von Muster GmbH` redigiert wird
- **THEN** lautet das Ergebnis `<redacted:vorname_1> <redacted:nachname_1> von <redacted:kunde_1>`.

#### Scenario: Längster Eintrag gewinnt
- **WHEN** Kunde `Anna Müller GmbH` und Vorname `Anna` konfiguriert sind und `Anna Müller GmbH` im Text steht
- **THEN** entsteht genau ein Treffer vom Typ `kunde`.

### Requirement: Namenslisten sind deterministisch und rückübersetzbar

Die Ersetzung durch die drei Listen MUST deterministisch sein: gleiche Eingabe und Konfiguration, Byte für Byte gleiche Ausgabe, unabhängig von der Reihenfolge der Einträge. Die Platzhalter MUST je Typ nummeriert werden. Die Rückübersetzung MUST sie wie alle Platzhalter behandeln, auch über Chunk-Grenzen im Stream und in `input_json_delta`.

#### Scenario: Reihenfolge der Einträge
- **WHEN** dieselben Einträge in zwei Reihenfolgen konfiguriert werden
- **THEN** ist die Ausgabe gleich.

#### Scenario: Rückweg im Stream
- **WHEN** ein Antwort-Stream `<redacted:kunde_1>` auf zwei Chunks verteilt zurückliefert
- **THEN** erscheint der Kundenname im Klartext, ohne Fragment des Platzhalters.

### Requirement: Namenslisten verraten nichts in Regelname und Maske

Der Regelname eines Treffers MUST die Liste nennen und nie den Eintrag. Die Maske eines Treffers der drei Typen MUST nur die Länge zeigen (`••• (N)`), nie ein Zeichen des Eintrags.

#### Scenario: Maske ohne erstes Zeichen
- **WHEN** ein Treffer vom Typ `nachname` maskiert wird
- **THEN** zeigt die Maske nur die Länge.

### Requirement: Maske je Treffer
Die Redact-Stufe SHALL für jeden Treffer eine Maske aus Regel, Typ und Klartext berechnen und am Treffer ablegen. Die Maske MUST deterministisch sein, höchstens ein Klarzeichen je geheimem Segment und insgesamt höchstens ein Viertel der Runen des Werts als Klarzeichen zeigen und höchstens 64 Bytes lang sein. Wortliste, Passwörter in Verbindungs-URLs, private Keys und heuristische Key-Treffer MUST ohne Klarzeichen maskiert werden (nur Länge). Der Body an Upstream und der Diff MUST unverändert bleiben.

#### Scenario: E-Mail
- **WHEN** die Adresse `nora.beispiel@firma-beispiel.de` redigiert wird
- **THEN** trägt der Treffer die Maske `n•••@f•••.de`

#### Scenario: API-Key mit bekanntem Präfix
- **WHEN** ein Anthropic-Key der Länge 108 mit Präfix `sk-ant-api03-` redigiert wird
- **THEN** besteht die Maske aus dem Präfix, einem Zeichen, `•••` und der Länge `(108)`

#### Scenario: Wortlisten-Eintrag
- **WHEN** der Wortlisten-Eintrag `Projekt Falke` redigiert wird
- **THEN** lautet die Maske `••• (13)` und enthält kein Zeichen des Eintrags

#### Scenario: Gleicher Wert, gleiche Maske
- **WHEN** derselbe Wert zweimal in einer Anfrage oder in zwei Anfragen redigiert wird
- **THEN** sind beide Masken bytegleich und der Upstream-Body ist derselbe wie ohne Maske

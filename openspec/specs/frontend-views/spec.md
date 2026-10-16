# frontend-views Specification

## Purpose
Die Bereiche Live, Kosten, Clients und Accounts des eingebetteten Frontends mit Admin-Token-Login, responsiv und mit Dark Mode.

## Requirements

### Requirement: Anmeldung mit dem Admin-Token

Das Frontend SHALL beim ersten Aufruf nach dem Admin-Token fragen, es lokal speichern und bei jeder API-Anfrage als Bearer-Token senden, auch für den Live-Stream. Das Token MUST NOT in einer URL erscheinen. Bei 401 MUST es verworfen und neu abgefragt werden.

#### Scenario: Abgelaufenes Token
- **WHEN** die API mit 401 antwortet
- **THEN** zeigt das Frontend wieder die Token-Abfrage.

### Requirement: Bereiche Live, Kosten, Clients und Accounts

Das Frontend SHALL die vier Bereiche mit echten Daten der Dashboard-API und der Client-API zeigen: Live (Anfragen mit Status, Modell, Tokens, Ersparnis, live aktualisiert), Kosten & Performance (Dashboard je Account und Details, siehe „Bereich Kosten & Performance“), Clients (erzeugen, sperren, rotieren, Setup-Snippet) und Accounts (aktiver Account, Health beider Dienste).

#### Scenario: Neuer Client
- **WHEN** der Admin einen Client anlegt
- **THEN** zeigt das Frontend den Key genau einmal mit einem kopierfertigen Snippet aus `ANTHROPIC_BASE_URL` und `ANTHROPIC_AUTH_TOKEN`.

#### Scenario: Live-Aktualisierung
- **WHEN** ein `trace`-Event eintrifft
- **THEN** erscheint die Anfrage oben in der Live-Liste oder wird dort aktualisiert, ohne die Seite neu zu laden.

### Requirement: Abo-Limits in Accounts

Die Accounts-Ansicht SHALL für das 5-Stunden-Fenster und die Woche je die Auslastung in Prozent als Balken, den Status als runden Indikator und die Zeit bis zum Reset mit Uhrzeit zeigen. Ohne Messung MUST sie „Noch keine Daten“ zeigen, statt Werte zu raten.

#### Scenario: Limits vorhanden
- **WHEN** `/api/status` `five_hour.utilization: 0.42` mit einem Reset in 2 h 13 min liefert
- **THEN** zeigt Accounts „42 %“ und „zurückgesetzt in 2 h 13 min“.

#### Scenario: Keine Daten
- **WHEN** `limits` `null` ist
- **THEN** zeigt Accounts „Noch keine Daten“.

### Requirement: Startseite Übersicht

Die Übersicht SHALL auf einen Blick zeigen: die Abo-Limits als Karte (5 Stunden und Woche je mit Prozent, Balken und „zurückgesetzt in …“ mit Datum), die Kennzahlen des heutigen UTC-Tages aus `/api/costs?days=1` (Anfragen, Tokens ein und aus, Cache-Trefferquote, gesparte Tokens), die Health von tollgate und CLIProxyAPI mit dem aktiven Account sowie die letzten 10 Anfragen mit Link ins Detail. Fehlt ein Wert, MUST sie „unbekannt“ oder „Noch keine Daten“ zeigen statt zu raten. Die Limit-Karte MUST als eigene Komponente für genau ein Abo nutzbar sein.

#### Scenario: Limits vorhanden
- **WHEN** `/api/status` `five_hour.utilization: 0.42` mit einem Reset in 2 h 13 min liefert
- **THEN** zeigt die Karte „42 %“ und „zurückgesetzt in 2 h 13 min“ mit Datum.

#### Scenario: Keine Limits
- **WHEN** `limits` `null` ist
- **THEN** zeigt die Karte „Noch keine Daten“ und keinen Balken.

#### Scenario: Letzte Anfrage öffnen
- **WHEN** der Nutzer auf eine der letzten Anfragen klickt
- **THEN** öffnet sich Live mit dieser Anfrage aufgeklappt.

### Requirement: Filter-Bereich

Der Bereich Filter SHALL die aktive Redactor-Konfiguration als Liste zeigen (eingebaute Filter als Schalter, eigene Regeln, Wortliste nur maskiert, Allowlist) und einen YAML-Entwurf bearbeiten lassen, entweder über das Formular oder über einen CodeMirror-Editor. Beide Wege MUST denselben Entwurf ändern und über denselben Import-Endpunkt speichern. Der Editor MUST live gegen das von `GET /api/config/schema` ausgelieferte Schema validieren. Vor dem Speichern MUST ein Dry-Run des aktuellen Entwurfs mit Vorschau pro Fixture (aktiv gegenüber Entwurf) stattgefunden haben; Validierungsfehler MUST mit Pfad, Zeile und Spalte erscheinen. Der Bereich SHALL einen Live-Test (Text einfügen, Treffer und redigierten Text sehen), die Versionshistorie mit Diff, Aktivieren und Rollback sowie Import aus Datei und Export bieten. Ein Export mit Wortliste MUST vorher die Warnung „enthält die vertrauliche Wortliste – nicht committen“ bestätigen lassen. Solange der Nutzer die Wortliste nicht ersetzt, MUST der Entwurf sie per `words_from` übernehmen, statt maskierte Einträge zu senden.

#### Scenario: Formular und YAML gleichwertig
- **WHEN** der Nutzer im Formular einen Schalter ändert, eine eigene Regel und einen Allowlist-Eintrag hinzufügt und speichert
- **THEN** sendet das Frontend genau den YAML-Text aus `web/src/testdata/form-config.yaml` an den Import-Endpunkt, und der Editor zeigt diesen Text.

#### Scenario: Speichern erst nach Dry-Run
- **WHEN** der Entwurf geändert wurde und der Dry-Run Validierungsfehler meldet
- **THEN** zeigt der Bereich die Fehler mit Zeile und Spalte, und Speichern ist gesperrt.

#### Scenario: Wortliste bleibt maskiert
- **WHEN** die aktive Version eine Wortliste hat und der Bereich geladen wird
- **THEN** erscheinen nur maskierte Einträge, und Dry-Run und Import senden `words_from` mit leerer Liste.

#### Scenario: Export mit Wortliste
- **WHEN** der Nutzer den Export mit Wortliste wählt
- **THEN** erscheint die Warnung, und erst nach Bestätigung wird `export?words=plain` abgerufen.

#### Scenario: Rollback
- **WHEN** der Nutzer in der Versionshistorie bei einer älteren Version „Zurückrollen“ wählt
- **THEN** ruft das Frontend `POST /api/config/versions/{id}/rollback` auf und zeigt die neue aktive Version.

### Requirement: Stufen-Timeline in Live

Die aufgeklappte Detailansicht einer Anfrage in Live SHALL ihre Stufen aus `GET /api/traces/{id}` als Timeline zeigen: pro Stufe Dauer, Tokens vorher und nachher, Hashes, Konfigurationsversion, Treffer und den gespeicherten Diff. Der Rückweg MUST als eigene Zeile erscheinen.

#### Scenario: Timeline einer redigierten Anfrage
- **WHEN** der Nutzer eine Anfrage mit Stufen `intake` und `redact` aufklappt
- **THEN** zeigt die Timeline beide Stufen mit Tokens vorher → nachher, den Diff der Redact-Stufe und eine Zeile „Rückweg“.

### Requirement: Letzte Redactions auf der Übersicht

Die Übersicht SHALL die letzten 10 Redactions aus `GET /api/redactions?limit=10` zeigen: Zeit, Client, Typ, Platzhalter und Regel, mit Link ins Live-Detail der Anfrage. Ohne Treffer MUST „Noch keine Redactions“ erscheinen.

#### Scenario: Panel mit Treffern
- **WHEN** die API zwei Treffer liefert
- **THEN** zeigt das Panel beide mit Platzhalter und Typ und verlinkt `#/live/<trace-id>`.

### Requirement: Vorzeichen der Ersparnis in Live
Die Live-Liste SHALL die Ersparnis einer Anfrage als Token-Delta mit genau einem Vorzeichen zeigen: positive Ersparnis mit `−`, negative Ersparnis (Mehrverbrauch) mit `+`, jeweils gefolgt vom deutsch formatierten Betrag. Ohne Ersparnis (0 oder fehlend) SHALL ein Strich `–` erscheinen.
#### Scenario: Positive Ersparnis
- **WHEN** eine Anfrage 1234 Tokens gespart hat
- **THEN** zeigt Live „−1.234“
#### Scenario: Negative Ersparnis
- **WHEN** die Redaction den Text um 12 Tokens verlängert hat (Ersparnis −12)
- **THEN** zeigt Live „+12“ und nie „−-12“
#### Scenario: Keine Ersparnis
- **WHEN** die Ersparnis 0 ist oder fehlt
- **THEN** zeigt Live „–“

### Requirement: Abo-Karten auf der Übersicht
Ist die Account-Verwaltung verfügbar (`account_management`), SHALL die Startseite statt der einzelnen Limit-Karte je gespeichertem Abo dieselbe Abo-Karte wie der Bereich Accounts zeigen, ohne Aktionen und das aktive Abo zuerst. Ohne Account-Verwaltung MUST die bisherige Limit-Karte bleiben.

#### Scenario: Zwei Abos
- **WHEN** `/api/status` `account_management: true` liefert und zwei Abos gespeichert sind
- **THEN** zeigt die Übersicht zwei Abo-Karten mit 5-Stunden- und Wochenbalken, das aktive zuerst, und keinen Knopf „Aktivieren“

### Requirement: Wortliste ergänzen und einzeln entfernen

Der Abschnitt Wortliste im Filter-Bereich SHALL ein Feld „Wörter hinzufügen“ (ein Eintrag je Zeile oder kommagetrennt) und an jedem maskierten Eintrag „Entfernen“ bieten; „Wortliste ersetzen“ MUST erhalten bleiben. Hinzugefügte Wörter MUST als `words_add`, entfernte Einträge einer übernommenen Liste MUST als Index in `words_remove` neben `words_from` gesendet werden. Die Liste MUST weiterhin nur maskiert erscheinen, auch für neu hinzugefügte Wörter, und der YAML-Entwurf MUST NOT die übernommene Liste oder maskierte Einträge enthalten. Jede Operation MUST den Dry-Run ungültig machen, Speichern bleibt bis zum nächsten erfolgreichen „Prüfen“ gesperrt. Alle Bedienelemente MUST per Tastatur erreichbar sein.

#### Scenario: Wörter hinzufügen
- **WHEN** die aktive Version 2 eine Wortliste hat und der Nutzer `Neu AG, Zweite KG` hinzufügt, prüft und speichert
- **THEN** senden Dry-Run und Import `words_from: 2` und `words_add: ["Neu AG", "Zweite KG"]`, und die Liste zeigt die neuen Einträge nur maskiert.

#### Scenario: Eintrag entfernen
- **WHEN** der Nutzer beim zweiten Eintrag „Entfernen“ wählt, prüft und speichert
- **THEN** senden Dry-Run und Import `words_remove: [1]`, und der Eintrag verschwindet aus der Anzeige.

#### Scenario: Operation macht Dry-Run ungültig
- **WHEN** nach erfolgreichem „Prüfen“ ein Wort hinzugefügt wird
- **THEN** ist „Speichern und aktivieren“ gesperrt.

### Requirement: Namenslisten maskiert im Filter-Bereich

Der Filter-Bereich SHALL neben der Wortliste die Abschnitte Kunden, Vornamen und Nachnamen zeigen, je mit Anzahl und maskierten Einträgen. Der Klartext MUST erst auf den Klick „Bearbeiten“ von `GET /api/config/lists` geladen werden (bei übernommener Version; sonst aus dem Entwurf) und nur im Komponentenzustand liegen.

#### Scenario: Maskiert ohne Klick
- **WHEN** die aktive Version Kunden hat und der Filter-Bereich geladen wird
- **THEN** zeigt der Abschnitt Kunden Anzahl und maskierte Einträge und es gibt keinen Aufruf von `/api/config/lists`.

### Requirement: Namenslisten im Textfeld bearbeiten

„Bearbeiten“ SHALL das Textfeld der gewählten Liste mit einem Eintrag je Zeile öffnen; „Übernehmen“ MUST Zeilen trimmen und leere Zeilen und Dubletten (gemäß `case_sensitive`) verwerfen. Bei übernommener Version MUST das Ergebnis als `lists_set` neben `words_from` gesendet werden, der YAML-Entwurf MUST die Listen dann nicht enthalten.

#### Scenario: Bearbeiten und Speichern
- **WHEN** der Nutzer „Bearbeiten“ wählt, im Textfeld eine Zeile ergänzt, übernimmt, prüft und speichert
- **THEN** wurde `/api/config/lists` genau einmal aufgerufen und Dry-Run und Import senden `words_from` und `lists_set` mit der vollständigen bearbeiteten Liste (nur diese).

#### Scenario: Leere Zeilen und Dubletten
- **WHEN** das Textfeld `A`, eine leere Zeile, `a` und `B ` enthält und `case_sensitive` aus ist
- **THEN** enthält `lists_set` genau `A` und `B`.

### Requirement: Namenslisten-Änderung braucht Prüfen und ist bedienbar

Jede Änderung an den Namenslisten MUST den Dry-Run ungültig machen; gespeichert wird erst nach „Prüfen“. Alle Bedienelemente MUST per Tastatur erreichbar sein; Dark Mode und schmale Breite MUST wie im Rest des Bereichs funktionieren.

#### Scenario: Änderung sperrt Speichern
- **WHEN** nach erfolgreichem „Prüfen“ eine Liste übernommen wird
- **THEN** ist „Speichern und aktivieren“ gesperrt.

### Requirement: Wortliste ersetzen erhält die Namenslisten

„Wortliste ersetzen“ und „Wortliste entfernen“ MUST die Kunden-, Vornamen- und Nachnamenlisten einer übernommenen Version erhalten.

#### Scenario: Wortliste ersetzen
- **WHEN** die übernommene Version Kunden hat und der Nutzer die allgemeine Wortliste ersetzt
- **THEN** bleibt `words_from` gesetzt und die Kunden gehen nicht verloren.

### Requirement: Bereich Kompression

Der Bereich „Kompression“ SHALL die Regeln der aktiven Konfigurationsversion in Prüfreihenfolge mit Strategie, Befehlspräfixen und Grenzen zeigen und eigene Regeln kennzeichnen. Er MUST anzeigen, ob die Compress-Stufe läuft. Ein Playground SHALL Befehl und Ausgabe annehmen und jeden Schritt mit Zeilen- und Bytezahl zeigen; die Schritte MUST inline aufklappen, das Ergebnis ist offen.

#### Scenario: Stufe aus
- **WHEN** die Compress-Stufe abgeschaltet ist
- **THEN** zeigt der Bereich „Aus“ und der Playground funktioniert trotzdem.

### Requirement: Performance-Ansicht

Das Frontend SHALL einen Bereich Performance mit Zeitraum-Wahl (Heute, 7 Tage, 30 Tage) zeigen: je Modell Anzahl, Anteil langsamer Anfragen, TTFT und Gesamtdauer als min / median / p90 / max, Ausgabe-Tokens pro Sekunde und aufklappbar einen dezenten Verlauf je Tag sowie den tollgate-Eigenanteil. Darunter SHALL eine Karte Redactor-Kosten (Redact-Stufe und Rückweg je median, p90, max in ms, Anteil an der Gesamtdauer, Bytes, Treffer, zusätzliche Tokens) mit dem Hinweis stehen, dass deterministische Platzhalter den Prompt-Cache nicht ändern und zusätzliche Tokens geschätzt sind. Fehlen Daten, MUST „Noch keine Daten“ erscheinen statt geratener Werte.

#### Scenario: Tabelle je Modell
- **WHEN** `/api/performance` ein Modell mit TTFT-Median 1 200 000 µs liefert
- **THEN** zeigt die Zeile dieses Modells für TTFT min, median „1,2 s“, p90 und max.

#### Scenario: Ohne Redactor-Daten
- **WHEN** `redactor` `null` ist
- **THEN** zeigt die Karte Redactor-Kosten „Noch keine Daten“.

### Requirement: Langsame Anfragen rot

Live (Liste und Inline-Detail) und die letzten Anfragen der Startseite SHALL eine Anfrage mit `perf.slow` mit rotem Statuspunkt zeigen, dessen Beschriftung die Erklärung trägt, z. B. „Gesamtdauer: erwartet ~4,2 s, gebraucht 9,8 s“. Das Inline-Detail SHALL zusätzlich TTFT, Tokens pro Sekunde, tollgate-Eigenanteil und die Erklärung als Text zeigen. Eine laufende oder fehlgeschlagene Anfrage MUST ihren bisherigen Punkt behalten. Ohne Urteil MUST das Detail „noch zu wenig Vergleichswerte“ zeigen und der Punkt MUST NOT rot sein.

#### Scenario: Verzögerte Anfrage
- **WHEN** ein `trace`-Event mit `perf.slow: true`, Median 4,2 s und Wert 9,8 s eintrifft
- **THEN** zeigt Live die Anfrage mit rotem Punkt und dem Text „erwartet ~4,2 s, gebraucht 9,8 s“.

#### Scenario: Zu wenig Daten
- **WHEN** `perf` eines abgeschlossenen Traces `unbekannt` meldet
- **THEN** ist der Punkt nicht rot, und das Detail zeigt „noch zu wenig Vergleichswerte“.

### Requirement: Speedtest im Frontend

Die Performance-Ansicht SHALL einen Knopf „Speed testen“ mit Modellauswahl bieten. Vor dem Start MUST ein Dialog bestätigen lassen, dass der Test Abo-Kontingent verbraucht; erst danach wird `POST /api/speedtest` mit `confirm: true` gesendet. Das Ergebnis SHALL je Modell ohne und mit Redact TTFT, Dauer und Tokens pro Sekunde neben dem üblichen Median zeigen.

#### Scenario: Abbrechen
- **WHEN** der Nutzer den Bestätigungsdialog abbricht
- **THEN** sendet das Frontend keine Anfrage.

#### Scenario: Ergebnis
- **WHEN** die API zwei Ergebnisse für ein Modell liefert
- **THEN** zeigt die Karte beide Varianten mit TTFT, Dauer und Tokens pro Sekunde und den üblichen Median daneben.

### Requirement: Masken in Timeline und Letzten Redactions
Die Redact-Stufe der Stufen-Timeline SHALL je Platzhalter eine Zeile mit Typ, Platzhalter und Maske zeigen, mehrfache Treffer desselben Platzhalters mit Anzahl. Das Panel „Letzte Redactions“ SHALL die Maske je Treffer zeigen. Fehlt die Maske (ältere Traces), MUST ein Strich `–` erscheinen.

#### Scenario: Timeline mit Masken
- **WHEN** die Redact-Stufe zwei Treffer `<redacted:email_1>` mit Maske `n•••@f•••.de` und einen Treffer `<redacted:api_key_1>` hat
- **THEN** zeigt die Timeline eine Zeile `email` · `<redacted:email_1>` · `n•••@f•••.de` · `2 ×` und eine Zeile für den API-Key mit seiner Maske

#### Scenario: Übersicht mit Maske
- **WHEN** `/api/redactions` einen Treffer mit Maske liefert und einen ohne
- **THEN** zeigt das Panel beim ersten die Maske und beim zweiten `–`

### Requirement: Volle Breite des Inhaltsbereichs
Der Inhaltsbereich neben der Sidebar SHALL die gesamte verfügbare Breite nutzen und keine feste Maximalbreite haben. Fließtext-Absätze SHALL für die Lesbarkeit auf etwa 75 Zeichen Zeilenlänge begrenzt bleiben; Tabellen, Listen und Kachel-Grids SHALL nicht begrenzt sein.
#### Scenario: Breiter Bildschirm
- **WHEN** das Frontend bei 1920 px Breite geöffnet ist
- **THEN** reicht der Inhalt jedes Bereichs bis zum rechten Seitenrand (abzüglich Padding)
#### Scenario: Langer Hinweistext
- **WHEN** ein Bereich einen Absatz mit Fließtext zeigt
- **THEN** ist dessen Zeilenlänge auf etwa 75 Zeichen begrenzt

### Requirement: Kachel-Grid der Übersicht
Die Abo-Kacheln der Übersicht SHALL als Grid mit `auto-fit` und `minmax` laufen, sodass bei 1440 px Breite drei Kacheln in einer Zeile stehen und auf schmalen Geräten (390 px) jede Kachel die volle Breite einnimmt und die Kacheln untereinander stehen.
#### Scenario: Drei Abos bei 1440 px
- **WHEN** drei Abos gespeichert sind und das Fenster 1440 px breit ist
- **THEN** stehen alle drei Abo-Kacheln in einer Zeile
#### Scenario: iPhone
- **WHEN** die Übersicht auf einem 390 px breiten Gerät geöffnet ist
- **THEN** stehen die Abo-Kacheln untereinander, ohne horizontal überzulaufen

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

### Requirement: Zeitmarke in den Limit-Fenstern

Die Limit-Karte der Übersicht (`LimitCard`) SHALL im Balken des 5-Stunden-Fensters und der Woche, die Abo-Nutzung (`AccountUsage`) im Balken des 5-Stunden-Fensters zusätzlich zur bestehenden Wochenmarke einen Strich für „jetzt“ zeigen. Seine Position SHALL der vergangene Anteil des Fensters sein: (jetzt − (Reset − Fensterlänge)) / Fensterlänge, mit 5 h bzw. 7 Tagen als Fensterlänge. Unter dem Balken SHALL „Strich = jetzt · X % des Fensters vergangen“ (5 Stunden) bzw. „Strich = jetzt · X % der Woche vergangen“ (Woche) stehen. Ohne gültigen Reset, bei abgelaufenem Reset, bei einem Reset weiter als die Fensterlänge entfernt oder ohne Messwert MUST NOT eine Marke erscheinen.

#### Scenario: 5-Stunden-Fenster zu einem Fünftel vergangen
- **WHEN** es 12:00 Uhr ist und das 5-Stunden-Fenster um 16:00 Uhr zurückgesetzt wird
- **THEN** steht der Strich bei 20 % des Balkens und darunter „Strich = jetzt · 20 % des Fensters vergangen“.

#### Scenario: Woche zur Hälfte vergangen in der Übersicht
- **WHEN** die Woche in 3 T 12 h zurückgesetzt wird
- **THEN** zeigt die Limit-Karte im Wochenbalken den Strich bei 50 % und „Strich = jetzt · 50 % der Woche vergangen“.

#### Scenario: Reset unbekannt oder abgelaufen
- **WHEN** der Reset des 5-Stunden-Fensters fehlt, ungültig ist oder in der Vergangenheit liegt
- **THEN** zeigt der 5-Stunden-Balken keinen Strich und keinen Text „Strich = jetzt“.

#### Scenario: Reset weiter entfernt als das Fenster
- **WHEN** der Reset des 5-Stunden-Fensters 6 h entfernt ist
- **THEN** zeigt der 5-Stunden-Balken keinen Strich.

### Requirement: Aktives Abo grün umrandet

Die aktive Abo-Karte SHALL eine dezente Outline in der Statusfarbe „ok“ (`--c-ok`) tragen, in der Limit-Karte der Übersicht und in der Abo-Karte (Übersicht und Accounts), im hellen wie im dunklen Modus. Inaktive Karten MUST NOT diese Outline tragen.

#### Scenario: Aktive Limit-Karte
- **WHEN** die Übersicht die Limit-Karte des aktiven Abos zeigt
- **THEN** trägt die Karte die Markierung „aktiv“ und eine grüne Outline statt des blassen Akzents.

#### Scenario: Inaktives Abo
- **WHEN** ein Abo nicht aktiv ist
- **THEN** trägt seine Karte keine grüne Outline.

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

### Requirement: Abo-Karten nach Verfügbarkeit

Die Übersicht SHALL die Abo-Karten in dieser Reihenfolge zeigen: das aktive Abo; Abos mit Wochenkontingent und freiem 5-h-Fenster, nach niedrigster Wochenauslastung und bei Gleichstand nach niedrigster 5-h-Auslastung; Abos mit Wochenkontingent und vollem 5-h-Fenster, nach nächstem 5-h-Reset; Abos mit verbrauchtem Wochenkontingent, nach Wochen-Reset; Abos ohne Nutzungsdaten. Ein Wochenkontingent MUST ab 99 % Auslastung im laufenden Fenster als verbraucht gelten. Ein Fenster mit Reset in der Vergangenheit MUST als neues Fenster mit 0 % zählen. Bei Gleichstand MUST die Reihenfolge nach E-Mail (sonst Name) eindeutig sein.

#### Scenario: Fenster des aktiven Abos voll
- **WHEN** das aktive Abo sein 5-h-Limit erreicht hat, ein Abo mit vollem 5-h-Fenster in 1 h zurückgesetzt wird und ein anderes mit 60 % Auslastung frei ist
- **THEN** steht nach dem aktiven Abo zuerst das freie, danach das volle.

#### Scenario: Niedrigste Woche zuerst
- **WHEN** zwei Abos ein freies 5-h-Fenster haben, eines mit 40 % und eines mit 70 % Wochenauslastung
- **THEN** steht das mit 40 % vorn.

#### Scenario: Woche bei 99 %
- **WHEN** ein Abo 99 % der Woche verbraucht hat und ein anderes ein volles 5-h-Fenster bei 50 % Woche
- **THEN** steht das Abo mit 99 % hinter dem mit vollem 5-h-Fenster.

#### Scenario: Wochenkontingent verbraucht
- **WHEN** ein Abo 100 % der Woche verbraucht hat
- **THEN** steht es hinter allen Abos mit Wochenkontingent.

### Requirement: Kompakte Abo-Karten

Eine Abo-Karte SHALL die E-Mail als Titel zeigen und den hinterlegten Namen als Tooltip; ohne E-Mail MUST der Name der Titel sein. Der Reset eines Fensters SHALL in derselben Zeile rechts neben der Prozentzahl stehen. Eine Textzeile zur Erklärung der Zeitmarke MUST NOT erscheinen; die Marke MUST ihr Label und ihren Tooltip behalten.

#### Scenario: Karte mit E-Mail
- **WHEN** ein Abo den Namen „Hauptabo“ und die E-Mail `a@example.com` hat
- **THEN** lautet der Titel `a@example.com`, der Name erscheint nur als Tooltip, und die E-Mail steht nicht ein zweites Mal auf der Karte.

### Requirement: Kosten im Abo-Fenster

Jede Abo-Karte der Übersicht SHALL unter dem 5-h- und dem Wochenfenster die Kosten und die Anzahl der Anfragen dieses Abos im laufenden Fenster zeigen. Gezählt MUST jede Stundenzeile des Abos ab der Stunde, in der das Fenster beginnt (Reset minus Fensterlänge). Fehlt für eine Stunde der Preis, MUST die Summe als Untergrenze („≥“) erscheinen. Ohne laufendes Fenster (Reset unbekannt, vorbei oder weiter als die Fensterlänge entfernt) MUST NOT ein Kostenwert erscheinen. Die Seite Accounts zeigt keine Fensterkosten.

#### Scenario: Kosten im 5-h-Fenster
- **WHEN** das 5-h-Fenster eines Abos um 14:45 UTC zurückgesetzt wird und das Abo um 08:00, 09:00 und 12:00 UTC Kosten hatte
- **THEN** zählt die Karte die Stunden 09:00 und 12:00, nicht 08:00.

### Requirement: Wochenvergleich auf der Übersicht

Die Übersicht SHALL einen Abschnitt „Diese Woche“ zeigen: Kosten, Anfragen, Tokens ein, Tokens aus, Cache-Trefferquote und Gespart der laufenden Kalenderwoche (UTC, ab Montag, bis heute), jeweils mit dem Wert der Vorwoche bis zum selben Wochentag und der Änderung (relativ, bei der Trefferquote in Prozentpunkten). Eine Tabelle SHALL alle sieben Wochentage mit Kosten und Tokens dieser Woche und der Vorwoche zeigen; Tage nach heute MUST leer bleiben. Die Summe der ganzen Vorwoche MUST sichtbar sein.

#### Scenario: Mittwoch
- **WHEN** heute Mittwoch ist und die Vorwoche am Montag $1, am Mittwoch $2 und am Freitag $4 kostete
- **THEN** vergleicht die Kachel „Kosten“ mit $3, die Tabelle zeigt Donnerstag bis Sonntag dieser Woche leer, und die Vorwochensumme beträgt $7.

### Requirement: Manueller Wechsel bei vollem Abo

Hat das aktive Abo im laufenden 5-h-Fenster 100 % oder im laufenden Wochenfenster mindestens 99 % erreicht, SHALL die Übersicht seine Karte rot umranden und „Limit erreicht“ zeigen. Das erste Abo in der Reihenfolge der Übersicht mit Nutzungsdaten, ohne erreichtes Limit, SHALL eine blau blinkende Umrandung, den Hinweis „als Nächstes“ und den Knopf „Account wechseln“ bekommen. Bei `prefers-reduced-motion` MUST die Umrandung ruhig bleiben. Der Wechsel MUST erst auf Knopfdruck über `POST /api/accounts/{id}/activate` erfolgen; danach SHALL die Übersicht das Ergebnis melden und neu laden. Hat das aktive Abo noch Luft oder ist kein Abo frei, MUST weder Signal noch Knopf erscheinen.

#### Scenario: Aktives Abo voll
- **WHEN** das aktive Abo A 100 % im 5-h-Fenster meldet, B ebenfalls voll ist und C 30 % meldet
- **THEN** ist A rot umrandet, C blau blinkend mit Knopf „Account wechseln“, und tollgate wechselt nicht von selbst.

#### Scenario: Wechsel auf Knopfdruck
- **WHEN** der Betreiber bei C „Account wechseln“ drückt
- **THEN** wird C aktiviert, die Übersicht meldet „C ist jetzt aktiv.“, und der Knopf verschwindet.

### Requirement: Kostenkurve mit einer Linie je Account

Die Kostenkurve SHALL je Account eine eigene Linie in einem gemeinsamen Diagramm auf gemeinsamer Skala zeigen. Die Linien MUST übereinander gezeichnet und MUST NOT gestapelt werden, sodass jede Linie die Kosten ihres Accounts zeigt. Die ersten fünf Accounts SHALL in fester Reihenfolge je eine eigene kategoriale Farbe bekommen, die in hellem und dunklem Modus für Farbsehschwäche geprüft ist; weitere Accounts MUST grau erscheinen statt eine Farbe zu wiederholen. Legenden der Accounts MUST dieselben Farben verwenden.

#### Scenario: Zwei Accounts mit gleichen Kosten
- **WHEN** zwei Accounts in derselben Stunde je $1 kosten
- **THEN** liegen beide Linien in dieser Stunde auf der Höhe von $1, in zwei Farben.

### Requirement: Letzte 5 Minuten auf der Übersicht
Die Übersicht SHALL im Abschnitt „Letzte 5 Minuten“ die Zahl der Anfragen aus `/api/calls`, den Ø der 10 Fenster davor und einen runden Statusindikator mit „über Ø“, „unter Ø“ oder „wie Ø“ zeigen. Der Abschnitt MUST mit den übrigen Daten der Übersicht neu laden (Intervall und „Aktualisieren“). Schlägt der Abruf fehl, MUST der Fehler in diesem Abschnitt stehen, ohne die anderen Abschnitte zu verbergen.

#### Scenario: Über dem Durchschnitt
- **WHEN** `/api/calls` `current: 4`, `average: 2.3` und `trend: "above"` liefert
- **THEN** zeigt der Abschnitt „4“, „Ø 2,3 je 5 min in den 50 min davor“ und einen Indikator „über Ø“.

#### Scenario: Unter dem Durchschnitt
- **WHEN** `/api/calls` `trend: "below"` liefert
- **THEN** zeigt der Indikator „unter Ø“.

#### Scenario: Fehler
- **WHEN** `/api/calls` mit einem Fehler antwortet
- **THEN** steht die Meldung im Abschnitt „Letzte 5 Minuten“ und die übrigen Abschnitte bleiben sichtbar.

### Requirement: Kennzahlen-Kacheln unter Kosten & Performance

Kosten & Performance SHALL die Kennzahlen als Kacheln mit einer großen Zahl und einer kleineren Zeile darunter zeigen: Tokens verarbeitet (darunter Ausgabe und Eingabe ungecacht), Cache-Ersparnis (darunter Cache gelesen), Von tollgate gespart und Schnellstes 5-h-Fenster. Die Fenster-Kachel MUST groß die hochgerechnete Zeit bis 100 % zeigen und darunter Anstieg, beobachtete Dauer, Ø Anfragen/min und den Account. Ohne Wert MUST sie „–“ und einen Hinweis zeigen, dass die Messung erst ab dem Deploy läuft.

#### Scenario: Fenster vorhanden
- **WHEN** `fastest_fill` 100 min bis 100 % bei 2,5 Anfragen/min meldet
- **THEN** zeigt die Kachel groß „1 h 40 min“ und darunter „0 → 100 % in 1 h 40 min · Ø 2,5 Anfragen/min“ mit dem Account-Namen.

#### Scenario: Noch keine Messung
- **WHEN** `fastest_fill` `null` ist
- **THEN** zeigt die Kachel „–“ und den Hinweis.

### Requirement: Einschätzung der Abo-Kapazität

Kosten & Performance SHALL eine Karte „Einschätzung“ zeigen. Bedarf pro Woche = `fills_per_day` × 7; Kapazität pro Woche = `accounts` × `windows_per_week`. Ein Fortschrittsbalken MUST den Bedarf als Anteil der Kapazität zeigen (über 100 % voll gefüllt), dazu das Urteil „reicht“ (unter 80 %), „knapp“ (80 bis 100 %) oder „reicht nicht“ (über 100 %) als Text mit rundem Statusindikator und, falls bekannt, `free_this_week` als „diese Woche noch … frei“. Kennzahlen MUST mit großer Zahl und kleiner Zeile erscheinen: Ø 5-h-Fenster je Wochenlimit (mit Zahl der Stichproben-Wochen und Preis eines vollen Fensters), gefüllte 5-h-Fenster pro Tag, Bedarf pro Woche, Kapazität mit Zahl der Accounts. Fehlt `windows_per_week` oder `fills_per_day`, MUST die Karte statt Balken und Urteil einen Hinweis zeigen, welche Messung noch fehlt.

#### Scenario: Accounts reichen nicht
- **WHEN** `fills_per_day` 5, `windows_per_week` 8 und `accounts` 4 sind
- **THEN** zeigt die Karte Bedarf 35 und Kapazität 32 pro Woche, den Balken voll und „4 Accounts reichen nicht“.

#### Scenario: Noch zu wenig gemessen
- **WHEN** `window_cost_nano_usd` `null` ist
- **THEN** zeigt die Karte keinen Balken und den Hinweis, dass noch kein 5-h-Fenster mit 30 Punkten Anstieg gemessen ist.

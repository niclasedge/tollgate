# store Specification

## Purpose
Der Store persistiert Clients, Konfigurationsversionen, Sitzungen und Traces in einer SQLite-Datei, ohne cgo und ohne Klartext-Geheimnisse.

## Requirements

### Requirement: SQLite ohne cgo

Der Store SHALL SQLite über `modernc.org/sqlite` und `database/sql` im WAL-Modus mit genau einer Schreibverbindung nutzen. Das Paket und `cmd/tollgate` MUST mit `CGO_ENABLED=0` bauen, und die Tests des Pakets MUST mit `CGO_ENABLED=0` laufen.

#### Scenario: Statischer Build
- **WHEN** `CGO_ENABLED=0 go build ./cmd/tollgate` ausgeführt wird
- **THEN** entsteht ein Binary ohne Fehler.

#### Scenario: Tests ohne cgo
- **WHEN** `CGO_ENABLED=0 go test ./internal/store/...` ausgeführt wird
- **THEN** sind alle Tests grün.

### Requirement: Migrationen

Der Store SHALL eingebettete, versionierte SQL-Migrationen in aufsteigender Reihenfolge mit einer Versionstabelle anwenden. Das Anwenden MUST idempotent und deterministisch sein: ein zweiter Lauf ändert nichts, und zwei frische Datenbanken haben dasselbe Schema. Eine nachträglich veränderte, bereits angewandte Migration MUST zu einem Fehler führen.

#### Scenario: Parallele Opener
- **WHEN** zwei Stores gleichzeitig dieselbe neue Datei öffnen
- **THEN** wird das Schema genau einmal angelegt und beide öffnen ohne Fehler.

#### Scenario: Zweiter Lauf
- **WHEN** `Open` zweimal auf derselben Datei aufgerufen wird
- **THEN** bleibt `schema_migrations` unverändert und es entsteht kein Fehler.

#### Scenario: Verändertes Migrationsskript
- **WHEN** die gespeicherte Prüfsumme einer angewandten Version von der eingebetteten Datei abweicht
- **THEN** schlägt `Open` mit einem Prüfsummenfehler fehl.

### Requirement: Dateirechte und Schreibsperre

Der Store SHALL die Datenbankdatei mit Modus 0600 anlegen (auch `-wal` und `-shm`) und eine bestehende, großzügiger berechtigte Datei auf 0600 setzen. Schreibtransaktionen MUST mit `BEGIN IMMEDIATE` starten, sodass parallele Schreiber, auch aus mehreren Store-Instanzen auf derselben Datei, warten statt mit `SQLITE_BUSY` zu scheitern. `NewConfigVersion.Seal` MUST NOT den Store aufrufen.

#### Scenario: Neue Datei
- **WHEN** `Open` eine neue Datei anlegt und geschrieben wurde
- **THEN** haben Datenbank, `-wal` und `-shm` den Modus 0600.

#### Scenario: Bestehende Datei mit 0644
- **WHEN** `Open` eine vorhandene Datei mit Modus 0644 öffnet
- **THEN** ist der Modus danach 0600.

#### Scenario: Parallele Schreiber
- **WHEN** zwei Store-Instanzen auf derselben Datei zusammen 40 Konfigurationsversionen parallel anlegen
- **THEN** gibt es keine Fehler und keine doppelten IDs.

### Requirement: Clients nur als Hash

Der Store SHALL Client-Keys ausschließlich als SHA-256-Hash mit kurzem Hinweis (`KeyHint`) speichern. Er MUST Hashes mit falscher Länge und zu lange Hinweise ablehnen, sodass ein Klartext-Key nicht gespeichert werden kann. Der Name eines Clients SHALL eindeutig sein.

#### Scenario: Klartext-Key als Hinweis
- **WHEN** ein 46 Zeichen langer `tg_`-Key als `KeyHint` übergeben wird
- **THEN** lehnt der Store mit `ErrInvalid` ab.

#### Scenario: Sperren und Rotieren
- **WHEN** ein Client gesperrt oder rotiert wird
- **THEN** ändert sich Status bzw. Hash und `rotated_at`, und der alte Hash findet keinen Client mehr.

### Requirement: Unveränderliche Konfigurationsversionen

Der Store SHALL Konfigurationsversionen mit aufsteigender ID, Autor, Kommentar, maskiertem Inhalt, opakem Chiffrat der Wortliste und Diff speichern. Versionen MUST unveränderlich sein: UPDATE und DELETE sind verboten. Genau eine Version ist aktiv.

#### Scenario: Versionen sind append-only
- **WHEN** ein SQL-UPDATE oder -DELETE auf `config_versions` ausgeführt wird
- **THEN** schlägt es fehl.

#### Scenario: INSERT OR REPLACE
- **WHEN** ein `INSERT OR REPLACE` auf eine bestehende Versions-ID ausgeführt wird
- **THEN** schlägt es fehl und die Version bleibt unverändert.

#### Scenario: Versiegeln mit Versions-ID
- **WHEN** eine Version mit `Seal`-Funktion angelegt wird
- **THEN** erhält `Seal` die ID, unter der die Version gespeichert wird, und das Ergebnis liegt als `words_enc` vor.

### Requirement: Sitzungs-Pinning

Der Store SHALL beim ersten Request einer Sitzung die Konfigurationsversion festhalten und bei späteren Requests unverändert zurückgeben, auch wenn inzwischen eine andere Version aktiv ist. Sitzungen ohne Aktivität seit einer vorgegebenen Schwelle MUST aufgeräumt werden können.

#### Scenario: Neue Version nach Pinning
- **WHEN** eine Sitzung mit Version 1 gepinnt ist und `PinSession` mit Version 2 erneut aufgerufen wird
- **THEN** bleibt die zurückgegebene Version 1.

### Requirement: Traces ohne Klartext

Der Store SHALL Traces mit Stufen-Messwerten (Dauer, Bytes und Tokens vorher/nachher, SHA-256 von Ein- und Ausgabe, redigierter Diff, Konfigurationsversion, Treffer als Offset, Länge, Typ, Regel) und `usage` (`input`, `output`, `cache_read`, `cache_creation`) speichern. Die Typen MUST keine Felder für Bodies oder Werte enthalten, und Hash-, Diff- und Bezeichnerfelder MUST validiert werden.

#### Scenario: Trace mit Stufen
- **WHEN** ein Trace mit mehreren Stufen geschrieben und gelesen wird
- **THEN** stimmen Stufen in Reihenfolge, Messwerte und Treffer überein, und `FinishTrace` ergänzt Status und `usage`.

#### Scenario: Kein Klartext in der Datei
- **WHEN** nur über die Store-API geschriebene Daten vorliegen und die DB-Datei samt WAL nach einem bekannten Geheimnis durchsucht wird, das nie übergeben wurde
- **THEN** gibt es keinen Treffer.

#### Scenario: Modellname mit Klammern
- **WHEN** ein Trace mit Modell `claude-opus-4-1[1m]` geschrieben wird
- **THEN** wird er mit diesem Modellnamen gespeichert.

#### Scenario: Ungültiges Modell oder Endpoint
- **WHEN** `model` oder `endpoint` (untrusted Client-Eingabe) kein gültiger Bezeichner ist
- **THEN** wird der Trace mit `unknown` gespeichert, und der Originalwert gelangt nie in die Datenbank.

#### Scenario: Keyset-Pagination
- **WHEN** fünf Traces mit identischer Startzeit mit Limit 2 über den Cursor `(Before, BeforeID)` seitenweise gelesen werden
- **THEN** ergeben alle Seiten zusammen genau diese fünf Traces ohne Duplikate.

#### Scenario: Wertebereiche
- **WHEN** `StartedAt` leer ist, ein Zähler negativ oder der Status außerhalb 100 bis 599 liegt
- **THEN** lehnt der Store mit `ErrInvalid` ab.

#### Scenario: Positivkontrolle des Leck-Tests
- **WHEN** ein Marker bewusst über das erlaubte Freitextfeld `Diff` geschrieben wird
- **THEN** findet die Dateisuche ihn, während nie übergebene Geheimnisse nicht gefunden werden.

#### Scenario: Ungültiger Hash
- **WHEN** ein Hash kein 64-stelliger Hex-String ist
- **THEN** lehnt der Store mit `ErrInvalid` ab.

### Requirement: Rechte vorhandener Sidecar-Dateien

Der Store SHALL beim Öffnen vorhandene `-wal`- und `-shm`-Dateien, die gruppen- oder weltweit zugänglich sind, auf Modus 0600 setzen.

#### Scenario: Alte Installation mit 0644-Sidecars
- **WHEN** `-wal` und `-shm` mit Modus 0644 neben der Datenbank liegen und der Store geöffnet wird
- **THEN** haben beide danach den Modus 0600.

### Requirement: Trace-Liste ohne Temp-Sort

Die Abfrage von `ListTraces` SHALL für alle Filterkombinationen (mit und ohne `ClientID`, mit und ohne Cursor) über einen Index sortiert werden. Der Query-Plan MUST keinen `TEMP B-TREE` enthalten.

#### Scenario: Keyset-Abfrage mit Client-Filter
- **WHEN** `EXPLAIN QUERY PLAN` für `ClientID`, `Before` und `BeforeID` ausgeführt wird
- **THEN** nutzt der Plan `traces_client` und keinen `TEMP B-TREE`.

#### Scenario: Keyset-Abfrage ohne Client-Filter
- **WHEN** `EXPLAIN QUERY PLAN` für `Before` und `BeforeID` ohne `ClientID` ausgeführt wird
- **THEN** nutzt der Plan `traces_started_at` und keinen `TEMP B-TREE`.

### Requirement: Paralleles Öffnen einer frischen Datei

`Open` SHALL auch dann erfolgreich sein, wenn mehrere Stores gleichzeitig dieselbe neue Datei öffnen, und ein kurzzeitiges `SQLITE_BUSY` beim ersten Verbindungsaufbau mit Wiederholung überbrücken.

#### Scenario: Vier gleichzeitige Opener
- **WHEN** vier Stores gleichzeitig dieselbe neue Datei öffnen
- **THEN** sind alle `Open` erfolgreich und `schema_migrations` enthält jede Version genau einmal.

### Requirement: Platzhalter in Treffern und letzte Treffer

Ein gespeicherter Treffer SHALL zusätzlich seinen Platzhalter enthalten. Der Store MUST nur einen leeren Wert oder einen Platzhalter der Grammatik `<redacted:TYP_N>` (ADR-0006) annehmen und andere Werte mit `ErrInvalid` ablehnen. `RecentRedactions(limit)` SHALL die letzten Treffer über alle Traces liefern, neueste Anfrage zuerst, innerhalb einer Anfrage in Stufen- und Dokumentreihenfolge, mit Zeit, Trace-ID, Client-ID und -Name, Stufe, Typ, Regel und Platzhalter.

#### Scenario: Klartext im Platzhalterfeld
- **WHEN** ein Treffer mit dem Platzhalter `alice@example.com` gespeichert werden soll
- **THEN** lehnt der Store den Trace mit `ErrInvalid` ab.

#### Scenario: Letzte Treffer
- **WHEN** drei Traces mit zusammen zwölf Treffern gespeichert sind und `RecentRedactions(10)` gelesen wird
- **THEN** kommen zehn Treffer, zuerst die der neuesten Anfrage in Dokumentreihenfolge, jeweils mit Platzhalter und Client-Name.

### Requirement: Performance-Messwerte im Trace

Eine neue Migration SHALL `traces` um `ttft_us`, `return_us` und `return_replacements` (je `INTEGER`, `NULL` = unbekannt) erweitern, einen Index über `(model, started_at DESC, id DESC)` anlegen und die Tabelle `trace_perf` (`trace_id`, `metric` aus `ttft`/`duration`, `class`, `samples`, `median_us`, `p90_us`, `verdict` aus `normal`/`langsam`/`unbekannt`, Primärschlüssel `(trace_id, metric)`, Löschen kaskadiert) anlegen. `FinishTrace` SHALL die neuen Werte übernehmen; negative Werte, unbekannte `metric`-, `class`- oder `verdict`-Werte MUST mit `ErrInvalid` abgelehnt werden. Die neuen Spalten und die Tabelle MUST NOT Inhalte aufnehmen, nur Zahlen und feste Bezeichner.

#### Scenario: Migration auf bestehender Datenbank
- **WHEN** eine Datenbank mit Traces aus früheren Versionen migriert wird
- **THEN** bleiben alle Traces erhalten, und ihre neuen Spalten sind `NULL`.

#### Scenario: Werte schreiben und lesen
- **WHEN** `FinishTrace` mit `ttft_us`, `return_us` und `return_replacements` aufgerufen wird
- **THEN** liefert das Lesen des Traces dieselben Werte.

#### Scenario: Ungültiges Urteil
- **WHEN** ein Urteil mit `verdict = 'rot'` oder negativem `samples` geschrieben wird
- **THEN** lehnt der Store mit `ErrInvalid` ab.

### Requirement: Vergleichsabfrage

Der Store SHALL für ein Modell, einen Zeitpunkt und eine Messgröße die Vergleichswerte nach der `performance`-Spec liefern: Fenster `[t − 14 Tage, t)`, nur erfolgreiche `messages`-Traces mit Usage und Messgröße, ohne Speedtest-Client, höchstens 500, neueste zuerst nach `started_at DESC, id DESC`, zusammen mit den Größen für die Klassen.

#### Scenario: Fenster und Filter
- **WHEN** Traces vor 15 Tagen, mit Status 500, mit `error_kind`, ohne Usage, vom Speedtest-Client und mit späterem Start vorliegen
- **THEN** gehört keiner davon zu den Vergleichswerten.

#### Scenario: Obergrenze
- **WHEN** 600 passende Traces vorliegen
- **THEN** liefert die Abfrage genau die 500 neuesten.

### Requirement: Reservierter Speedtest-Client

Die Migration SHALL einen Client mit ID `speedtest`, Namen `tollgate:speedtest`, Schlüssel-Hash aus 32 Nullbytes und Status `gesperrt` anlegen. Der Store MUST das Anlegen von Clients mit dieser ID oder einem Namen mit Präfix `tollgate:` sowie Rotation und Statusänderung dieses Clients ablehnen. Eine Schlüsselprüfung MUST für diesen Client immer scheitern.

#### Scenario: Name reserviert
- **WHEN** ein Client `tollgate:test` angelegt werden soll
- **THEN** lehnt der Store mit `ErrInvalid` ab.

#### Scenario: Keine Anmeldung als Speedtest
- **WHEN** ein beliebiger Schlüssel geprüft wird
- **THEN** führt er nie zur Client-ID `speedtest`.

### Requirement: Maske in gespeicherten Treffern
Ein gespeicherter Treffer SHALL zusätzlich seine Maske enthalten, und `RecentRedactions` SHALL sie mitliefern. Der Store MUST nur eine leere Maske oder eine Maske mit mindestens einem `•`, gültigem UTF-8, ohne Steuerzeichen und mit höchstens 64 Bytes annehmen und andere Werte mit `ErrInvalid` ablehnen. Treffer, die vor der Maske gespeichert wurden, liefern eine leere Maske.

#### Scenario: Klartext im Maskenfeld
- **WHEN** ein Treffer mit der Maske `geheim123` gespeichert werden soll
- **THEN** lehnt der Store den Trace mit `ErrInvalid` ab

#### Scenario: Maske in den letzten Treffern
- **WHEN** ein Treffer mit Maske `n•••@f•••.de` gespeichert und `RecentRedactions(10)` gelesen wird
- **THEN** trägt der Treffer genau diese Maske

### Requirement: Tokens/s-Urteil speichern

Der Store SHALL das Urteil Tokens pro Sekunde in einer Tabelle `trace_tps` mit Trace-ID, Klasse, Anzahl Vergleichswerte, `tps_milli`, p10, Median, p90, Abweichung in Promille und Urteil speichern. Das Urteil MUST aus der festen Liste `schneller`, `üblich`, `langsamer`, `unbekannt` stammen, Zahlen MUST NOT negativ sein, und die Zeile MUST mit dem Trace gelöscht werden. Die Tabelle MUST NOT Inhalte der Anfrage enthalten. Fehlt eine Zeile, SHALL das Lesen des Traces kein Urteil liefern.

#### Scenario: Schreiben und Lesen
- **WHEN** für einen Trace ein Urteil `langsamer` mit 30 Vergleichswerten gespeichert wird
- **THEN** liefert das Lesen des Traces dieses Urteil mit denselben Zahlen.

#### Scenario: Ungültiges Urteil
- **WHEN** ein Urteil außerhalb der festen Liste gespeichert werden soll
- **THEN** lehnt der Store es mit `ErrInvalid` ab.

#### Scenario: Kaskade
- **WHEN** ein Trace gelöscht wird
- **THEN** ist auch seine Zeile in `trace_tps` gelöscht.

### Requirement: Kosten je Stunde

Der Store SHALL abgeschlossene Traces ab einem Zeitpunkt je UTC-Stunde (`YYYY-MM-DDTHH:00:00Z`), Account und Modell mit Anzahl und Usage zusammenfassen, sortiert nach Stunde, Account und Modell. Traces ohne Account MUST unter leerem Account erscheinen, Traces ohne Usage MUST als Anfrage ohne Tokens zählen, und laufende Traces MUST fehlen.

#### Scenario: Stundengrenze
- **WHEN** ein Trace um 04:59:59.999 UTC und einer um 05:00:00.000 UTC startet
- **THEN** liegt der erste in `…T04:00:00Z` und der zweite in `…T05:00:00Z`.

#### Scenario: Laufender Trace
- **WHEN** ein Trace noch keinen Status hat
- **THEN** erscheint er in keiner Stunde.

### Requirement: 5-h-Auslastung am Trace

Der Store SHALL am Trace die 5-h-Auslastung in Promille (`five_hour_permille`, ganzzahlig, nicht negativ) und den Reset-Zeitpunkt des 5-h-Fensters (`five_hour_reset`, Unix-Sekunden) speichern. Beide Werte MUST NULL bleiben, wenn die Antwort sie nicht enthält. Der Store MUST die Zeilen für die Fensterauswertung ab einem Zeitpunkt nach `started_at` sortiert liefern.

#### Scenario: Werte aus der Antwort
- **WHEN** ein Trace mit Auslastung 0,423 und Reset 1760000000 abgeschlossen wird
- **THEN** stehen am Trace 423 und 1760000000.

#### Scenario: Antwort ohne Limit-Header
- **WHEN** ein Trace ohne diese Werte abgeschlossen wird
- **THEN** bleiben beide Spalten NULL.

### Requirement: 7-Tage-Auslastung am Trace

Der Store SHALL am Trace die 7-Tage-Auslastung in Promille (`seven_day_permille`, ganzzahlig, nicht negativ) und den Reset-Zeitpunkt des 7-Tage-Fensters (`seven_day_reset`, Unix-Sekunden) speichern und in den Zeilen der Fensterauswertung liefern. Fehlen die Werte in der Antwort, MUST beide NULL bleiben.

#### Scenario: Werte aus der Antwort
- **WHEN** ein Trace mit 7-Tage-Auslastung 0,31 und Reset 1760500000 abgeschlossen wird
- **THEN** stehen am Trace 310 und 1760500000, und die Fensterauswertung liefert beide.

### Requirement: Preis-Grundlage in der Fensterauswertung

Die Zeilen der Fensterauswertung SHALL Modell und Usage (Eingabe, Ausgabe, Cache-Lesen, Cache-Schreiben; fehlende Werte als 0) des Traces liefern.

#### Scenario: Usage in der Zeile
- **WHEN** ein Trace mit Usage 5/7/11/13 abgeschlossen wird
- **THEN** liefert die Fensterauswertung Modell und genau diese Usage; ein Trace ohne Usage liefert 0.

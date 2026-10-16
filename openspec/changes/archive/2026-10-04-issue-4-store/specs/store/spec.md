# Spec Delta

## Purpose

Der Store persistiert Clients, Konfigurationsversionen, Sitzungen und Traces in einer SQLite-Datei, ohne cgo und ohne Klartext-Geheimnisse.

## ADDED Requirements

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

## ADDED Requirements

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

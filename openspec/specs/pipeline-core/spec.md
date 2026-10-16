# pipeline-core Specification

## Purpose
Der Pipeline-Kern führt Stufen in fester Reihenfolge aus, wendet ihre Ersetzungslisten an und hält pro Anfrage und Stufe einen nachvollziehbaren Trace fest.

## Requirements

### Requirement: Stage-Interface mit Ersetzungslisten

`pipeline.Stage` SHALL eine Methode `Process` mit Request-Kontext bereitstellen, die eine Liste von Ersetzungen oder einen Fehler liefert. Der Runner MUST die Ersetzungen mit `jsonsplice.Apply` anwenden; Stufen MUST NOT den Body direkt verändern.

#### Scenario: Roundtrip durch Stufen ohne Änderung
- **WHEN** ein Body durch Eingang und eine Stufe ohne Ersetzungen läuft
- **THEN** ist der Ausgabe-Body bytegleich zur Eingabe und `OriginalBody` unverändert.

### Requirement: Trace pro Anfrage und Stufe

Der Runner SHALL jeder Anfrage vor Stufe 0 eine Trace-ID (32 Hex-Zeichen aus `crypto/rand`) geben und pro Stufe Name, Dauer, geschätzte Tokens vorher und nachher, SHA-256 von Ein- und Ausgabe, Konfigurationsversion und die Ersetzungsliste erfassen. Der Trace MUST NOT Klartext-Originalwerte oder Client-Keys enthalten, und die Trace-ID MUST NOT in den weitergeleiteten Body gelangen.

#### Scenario: Zwei Stufen
- **WHEN** Eingang und eine ersetzende Stufe laufen
- **THEN** stimmt der Eingangs-Hash der zweiten Stufe mit dem Ausgangs-Hash der ersten überein, der Ausgangs-Hash der zweiten mit dem SHA-256 des Ergebnis-Bodys, und die Tokens nachher sind kleiner als vorher.

#### Scenario: Kein Klartext im Trace
- **WHEN** eine Stufe `my-secret-value` ersetzt
- **THEN** enthält der serialisierte Trace weder diesen Wert noch den Client-Key.

### Requirement: Abbruch bei Fehlern

Bei einem Stufenfehler oder ungültigen Ersetzungen SHALL der Runner abbrechen, die Folgestufen nicht ausführen, die Stufe im Trace als fehlgeschlagen markieren und einen `StageError` liefern. `ErrorResponse` MUST daraus einen Anthropic-Fehlerkörper `{"type":"error","error":{"type":…,"message":…}}` mit fester, plaintext-freier Meldung erzeugen. Als ungültig SHALL jede Ersetzung gelten, deren Bereich nicht exakt einem von `Scan` gemeldeten Ziel entspricht.

#### Scenario: Stufe schlägt fehl
- **WHEN** eine Stufe einen Fehler mit sensiblem Text liefert
- **THEN** antwortet `ErrorResponse` mit 500 `api_error` ohne diesen Text, und spätere Stufen laufen nicht.

#### Scenario: Ersetzung außerhalb eines Ziels
- **WHEN** eine Stufe eine Ersetzung für eine Thinking-Signatur, einen Member-Namen oder einen Bereich innerhalb eines Strings liefert
- **THEN** bricht der Runner mit 500 ab, und der Body bleibt unverändert.

### Requirement: Determinismus

Gleiche Eingabe und gleiche Stufen SHALL bytegleichen Ausgabe-Body und gleiche Hashes ergeben, auch bei parallelen Läufen unter `-race`.

#### Scenario: Parallele Läufe
- **WHEN** 100 Läufe parallel dieselbe Eingabe verarbeiten
- **THEN** ist der SHA-256 des Ergebnisses in allen Läufen gleich.

### Requirement: Eingangsstufe Client-Key

Stufe 0 SHALL den Client-Key aus `Authorization: Bearer` oder `x-api-key` über das Interface `KeyVerifier` prüfen. Fehlender oder ungültiger Key MUST mit 401 `authentication_error` abgelehnt werden, ein Prüferfehler mit 500 `api_error`. Die Prüfung MUST für alle Pfade gelten.

#### Scenario: Ungültiger Key
- **WHEN** ein unbekannter Key vorgelegt wird
- **THEN** endet die Anfrage mit 401 und besitzt trotzdem eine Trace-ID.

### Requirement: Eingangsstufe parst /v1/messages

Stufe 0 SHALL für `POST /v1/messages` und `POST /v1/messages/count_tokens` den Body strikt prüfen und Modell und Stream-Flag übernehmen. Ungültiges JSON MUST mit 400 `invalid_request_error` abgelehnt werden. Alle anderen Pfade und Methoden SHALL als Durchreich-Endpunkt unverändert und ungeparst bleiben.

#### Scenario: Ungültiges JSON
- **WHEN** `/v1/messages` einen abgeschnittenen Body erhält
- **THEN** antwortet die Pipeline mit 400 und dem Anthropic-Fehlerformat.

#### Scenario: Durchreich-Endpunkt
- **WHEN** `GET /v1/models` mit gültigem Key eintrifft
- **THEN** ist der Endpunkt `passthrough` und der Body wird nicht geparst.

### Requirement: Sitzungsschlüssel

Stufe 0 SHALL die Sitzungs-ID aus dem Header `X-Claude-Code-Session-Id`, sonst aus `metadata.user_id` ableiten und den Schlüssel als hex(SHA-256(client_id ‖ 0x00 ‖ session_id)) setzen; ohne Sitzungs-ID MUST der Schlüssel leer bleiben. Verschiedene Clients mit gleicher Sitzungs-ID MUST verschiedene Schlüssel erhalten. Die optionale Konfigurationsversion SHALL über `ConfigPinner` ermittelt werden.

#### Scenario: Header vor Metadaten
- **WHEN** Header und `metadata.user_id` unterschiedliche Sitzungs-IDs tragen
- **THEN** gilt die ID aus dem Header.

#### Scenario: Keine Sitzung
- **WHEN** weder Header noch auswertbare Metadaten vorhanden sind
- **THEN** bleibt der Sitzungsschlüssel leer.

### Requirement: Sitzungsquelle an echter Anfrage belegt

Der Sitzungsschlüssel SHALL aus einer aufgezeichneten, bereinigten Claude-Code-Anfrage (`testdata/fixtures/`) abgeleitet werden können, sowohl aus dem Header als auch, ohne Header, aus dem JSON-Objekt in `metadata.user_id`.

#### Scenario: Fixture mit Header
- **WHEN** die Fixture unverändert durch Stufe 0 läuft
- **THEN** ist der Sitzungsschlüssel nicht leer und die Quelle `header`.

#### Scenario: Fixture ohne Header
- **WHEN** der Header `X-Claude-Code-Session-Id` entfernt wird
- **THEN** entsteht derselbe Sitzungsschlüssel mit Quelle `metadata`.

### Requirement: Stufenbericht mit Diff und Treffern

Eine Stufe SHALL über `Request.Report` einen redigierten Diff und Treffer (Offset, Länge, Typ, Regel) melden können. Der Runner MUST den Bericht vor jeder Stufe zurücksetzen und ihn nur bei Erfolg in `StageTrace.Diff` und `StageTrace.Matches` übernehmen. Treffer MUST NOT den ersetzten Wert enthalten.

#### Scenario: Bericht im Trace
- **WHEN** eine Stufe einen Diff und einen Treffer meldet und die nächste Stufe nichts meldet
- **THEN** trägt nur der Trace-Eintrag der ersten Stufe Diff und Treffer.

### Requirement: Platzhalter-Zuordnung im Request

`Request.Placeholders` SHALL die Zuordnung Platzhalter → Klartext der Anfrage für den Rückweg tragen. Das Feld MUST bei JSON-Serialisierung ausgelassen werden und MUST NOT in den Trace gelangen.

#### Scenario: Serialisierung
- **WHEN** ein Request mit gefüllter Zuordnung als JSON serialisiert wird
- **THEN** enthält das Ergebnis keinen Klartext der Zuordnung.

### Requirement: Sitzungs-Pinning über den Store

`SessionPinner` SHALL `ConfigPinner` über den Store umsetzen: Ohne aktive Konfigurationsversion MUST er 0 liefern und nichts schreiben. Mit aktiver Version und Sitzungsschlüssel MUST er die Sitzung pinnen und die zuerst gespeicherte Version liefern. Ohne Sitzungsschlüssel MUST er die aktive Version liefern, ohne zu schreiben.

#### Scenario: Version wechselt mitten in der Sitzung
- **WHEN** eine Sitzung mit aktiver Version 1 beginnt und danach Version 2 aktiviert wird
- **THEN** erhält die Sitzung weiter Version 1, eine neue Sitzung Version 2.

#### Scenario: Keine aktive Version
- **WHEN** im Store keine Version aktiv ist
- **THEN** liefert der Pinner 0, und es entsteht keine Sitzungszeile.

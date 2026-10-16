# ADR-0021: Namenslisten, Klartext nur zum Bearbeiten

- **Status:** Angenommen (Entscheidung Niclas, 2026-10-15, Issue #143)
- **Datum:** 2026-10-15
- **Issue:** #143
- **Ergänzt:** [ADR-0009](0009-authentifizierung-und-geheimnisse.md), [ADR-0018](0018-maske-aufdecken-ram.md)

## Kontext

Neben der allgemeinen Wortliste (`words.list`, Platzhalter `word`) sollen Kunden, Vornamen
und Nachnamen getrennt gepflegt werden, mit eigenen Platzhaltern (`kunde`, `vorname`,
`nachname`). Solche Listen sind lang und ändern sich; sie nur maskiert anzusehen und per
Index zu entfernen (#119) reicht dafür nicht. Gewünscht ist ein Textfeld mit einem Eintrag je
Zeile, nachträglich editierbar.

Bisher gilt (AGENTS.md, ADR-0009): Die Wortliste ist selbst vertraulich, wird verschlüsselt
gespeichert, im Frontend nur maskiert angezeigt und nie geloggt. ADR-0018 hat das Aufdecken
einzelner Treffer als schmale Ausnahme erlaubt und die Wortliste (`word`) davon ausdrücklich
ausgenommen.

## Entscheidung

### Listen

- Drei neue Listen im Block `words`: `customers` (Platzhalter `kunde`), `first_names`
  (`vorname`), `last_names` (`nachname`). Sie teilen `case_sensitive` und `whole_word` mit
  `words.list`, haben dieselben Grenzen (5000 Einträge, je 1 bis 200 Zeichen) und dieselbe
  Treffersemantik (längster Eintrag zuerst, Allowlist, Unicode-Wortgrenzen).
- `words.list` bleibt unverändert, mit Platzhalter `word`. Keine Migration; alte YAML-Dateien
  und gespeicherte Versionen bleiben gültig.
- Der versiegelte Blob einer Version enthält ein JSON-Objekt mit allen vier Listen. Das alte
  Format (JSON-Array, nur `words.list`) wird weiter gelesen.
- Die Typen `kunde`, `vorname`, `nachname` sind für eigene Regeln **nicht** reserviert:
  bestehende Konfigurationen mit einer eigenen Regel dieses Typs bleiben gültig.
- Maske der drei Typen: nur die Länge (`••• (N)`), wie bei `word`.

### Neue Ausnahme: Klartext zum Bearbeiten

Die Regel „Wortliste im Frontend maskiert anzeigen“ gilt weiter, mit **einer** Ausnahme:
Auf den ausdrücklichen Klick „Bearbeiten“ bei Kunden, Vornamen oder Nachnamen lädt das
Frontend den Klartext dieser Listen einmal und zeigt ihn in einem Textfeld.

- Endpunkt `GET /api/config/lists?version=N` (ohne `version` die aktive) hinter dem
  Admin-Token (ADR-0009). Antwort `200` mit den drei Listen, `Cache-Control: no-store`.
  Unbekannte Version `404`, kein Schlüssel `422`, ungültige Nummer `400`.
- Die allgemeine Wortliste ist **nicht** Teil der Antwort und bleibt maskiert (Bearbeiten
  wie bisher mit `words_add`/`words_remove`).
- Geloggt wird nur die Versionsnummer, nie ein Eintrag. Der Klartext liegt in der DB nie
  unverschlüsselt, in keinem Trace, Diff oder Log, und im Frontend nur im Zustand der
  Komponente, solange das Textfeld offen ist oder eine Ersetzung noch nicht gespeichert ist.
  Er landet weder im YAML-Entwurf einer übernommenen Version noch in `localStorage`.
- Der Rückweg ist der bestehende Entwurfsweg: `lists_set` ersetzt einzelne Listen
  vollständig (Server trimmt, wirft leere Zeilen und Dubletten weg, prüft Grenzen), danach
  Prüfen (Dry-Run) → Speichern. Ein Weg ohne Klartext bleibt vorhanden: Export `words=plain`
  (mit Warnung, ADR-0008) ist unverändert.
- Aufdecken einzelner Treffer (ADR-0018) bleibt für die drei Typen ausgeschlossen, wie für
  `word` und `private_key`.

### Harte Regel

AGENTS.md ergänzt die Regel zur Wortliste um diese Ausnahme (ein Satz, Verweis auf dieses
ADR). Logs, DB, Traces, Fehlermeldungen und Diffs bleiben frei von Einträgen.

## Alternativen

- **Klartext immer mitliefern (z. B. in der Versionsansicht):** landet in jeder Antwort und im
  Browser-Cache. Verworfen.
- **Nur maskiert bearbeiten (Hinzufügen/Entfernen wie #119):** für lange Namenslisten
  unpraktisch; Ändern hieße Entfernen und Neueintippen. Verworfen.
- **Klartext in der DB:** widerspricht ADR-0009. Verworfen.
- **Eigene Endpunkte je Liste:** mehr Fläche ohne Nutzen, die Antwort ist klein. Verworfen.
- **Eigene Regeln mit den neuen Typen verbieten:** würde bestehende Konfigurationen brechen.
  Verworfen.

## Konsequenzen

- Wer das Admin-Token hat, kann die drei Listen im Klartext lesen. Das Token schützt ohnehin
  Konfiguration und Export mit Klartext (`words=plain`), der Zugewinn ist gering.
- Im Browser steht der Klartext für die Dauer der Bearbeitung im Speicher.
- Der Leck-Test (`internal/redactcfg`) prüft Antworten, Logs und Datenbankdateien auf
  Kanarienwerte aller vier Listen; der Endpunkt ist die einzige erlaubte Quelle.
- Aufdecken (ADR-0018) sollte `redact.IsListType` für den Ausschluss nutzen (hinzugefügt),
  damit neue Listentypen nicht einzeln nachgezogen werden müssen.

## Quellen

- ADR-0005 (Redactor), ADR-0006 (Platzhalter), ADR-0008 (Schema, Versionen),
  ADR-0009 (Verschlüsselung), ADR-0018 (Aufdecken), #119 (Operationen am Entwurf).
- `internal/redactcfg/lists.go`, `internal/redact/words.go`, `internal/redact/config.go`.

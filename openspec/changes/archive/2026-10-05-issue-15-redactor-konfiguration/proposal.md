# Proposal

Issue: [#15 Redactor-Konfiguration: Schema, YAML, Versionen, Hot Reload](https://github.com/niclasedge/tollgate/issues/15)

Geht zurück auf GitHub-Issue #15. Die Abhängigkeiten #4 (Store) und #12 (Redactor-Port)
sind geschlossen. Verbindlich sind [ADR-0008](../../../docs/adr/0008-konfigurationsschema.md)
(Schema, Import, Export, Versionen, Hot Reload), [ADR-0005](../../../docs/adr/0005-redactor-port.md)
(eigene Regex, Wortliste, Allowlist), [ADR-0006](../../../docs/adr/0006-platzhalter-sitzungen-rueckweg.md)
(Platzhalter-Grammatik, Sitzungs-Pinning) und [ADR-0009](../../../docs/adr/0009-authentifizierung-und-geheimnisse.md)
(Verschlüsselung der Wortliste, kein Klartext).

## Why

Der Redactor aus #12 kennt nur die eingebauten Schalter mit festen Standardwerten. Firmen-,
Kunden- und Projektnamen, eigene Kennungen und Ausnahmen lassen sich nicht pflegen. Ohne
versionierte Konfiguration gibt es weder Rollback noch Hot Reload, und eine laufende
Sitzung könnte ihre Regeln mitten im Verlauf ändern, was den Prompt-Cache bricht.

## What Changes

- JSON-Schema `config/schema/tollgate-config.v1.schema.json` (Draft 2020-12,
  `additionalProperties: false` überall), per `embed` im Binary.
- Neues Paket `internal/redactcfg`: YAML-Import mit Schema- und Semantikprüfung
  (Fehler mit Pfad, Zeile und Spalte, ohne Wortlisten-Einträge), deterministischer Export,
  Dry-Run auf `testdata/fixtures/`, Versionen in `config_versions` mit maskiertem Diff,
  Rollback als neue Version, Hot Reload über `atomic.Pointer`, Cache kompilierter
  Versionen und der Resolver für die Redact-Stufe aus #13.
- `internal/redact`: `Compile(Config)` mit eigenen Regex (Priorität 100, jedes Beispiel
  muss treffen), Wortliste (Priorität 110, `case_sensitive`, Unicode-bewusstes
  `whole_word`, Mehrwort mit flexiblem Whitespace) und Allowlist (exakter Trefferwert).
- Wortliste verschlüsselt über ein kleines `Sealer`-Interface; ohne Sealer wird eine
  Wortliste abgelehnt. Der Anschluss an `internal/cryptobox` folgt nach dem Merge von #28.
- Beim ersten Start legt tollgate die eingebaute Standardkonfiguration als Version 1 an und
  aktiviert sie, damit jede Sitzung von Anfang an eine echte Version pinnt.
- Admin-API unter `/api/config/…`: Schema, Dry-Run, Import, Aktivieren, Export, Versionen,
  Rollback, Live-Test.

## Capabilities

### New Capabilities

- `redaction-config`: versionierte Laufzeitkonfiguration mit Schema, Import, Export,
  Dry-Run, Rollback, Hot Reload, Verschlüsselung und Admin-API.

### Modified Capabilities

- `redaction`: eigene Regex, Wortliste und Allowlist.

## Impact

Branch `issue-15/redactor-konfiguration`, gemeinsamer Workspace. Berührte Dateien:

- neu: `internal/redactcfg/**`, `internal/redact/config.go`, `internal/redact/words.go`,
  `internal/redact/config_test.go`, `config/schema/tollgate-config.v1.schema.json`,
  `config/schema/schema.go` (ersetzt `config/schema/.gitkeep`), `fixtures.go` (Paket `tollgate` im Modul-Root, bettet
  `testdata/fixtures/*.json` ein), `testdata/fixtures/redact-dry-run-messages.json`,
  diese Change-Artefakte
- kleine Hunks: `internal/redact/rules.go` (Feld `find` im Typ `rule`),
  `internal/redact/redact.go` (Feld `allow`, `rl.matches`, Allowlist-Prüfung in `Find`),
  `testdata/fixtures/README.md` (Abschnitt zur neuen Fixture), `cmd/tollgate/main.go`
  (Manager öffnen, Routen registrieren), `go.mod`/`go.sum`
  (`github.com/santhosh-tekuri/jsonschema/v6`, laut ADR-0003)
- keine Migration: `config_versions` und `config_active` aus `0001_init.sql` reichen.

## Was ausdrücklich nicht

- Kein Frontend (Editor, Formular, Versionshistorie, Dry-Run-Vorschau): #16.
- Keine Verdrahtung des Resolvers in die Pipeline: #13 nutzt `Manager.Redactor`.
- Kein Anschluss an `internal/cryptobox` vor dem Merge von #28 (offene Aufgabe).
- Abschnitt `compress:` wird nur validiert und versioniert, nicht ausgewertet: #19.

# Proposal

Issue: [#1 Architektur und ADRs (Freigabe-Gate)](https://github.com/niclasedge/tollgate/issues/1)

## Why

tollgate hat noch keine verbindliche Architektur. Drei Grundsatzfragen beeinflussen alle 24 Folge-Issues:

- Wie wird CLIProxyAPI eingebunden?
- Was wird aus dem Rust-Redactor und aus Headroom übernommen?
- Wie bleibt das Prompt-Caching trotz Umschreiben der Anfragen stabil?

Laut `CLAUDE.md` darf vor einer Freigabe der Architektur kein Produktionscode entstehen. Dieser Change liefert die Entscheidungsgrundlage für diese Freigabe.

## What Changes

- Neu: `docs/ARCHITECTURE.md`. Enthält Datenfluss, Komponenten, Pipeline-Stufen, Rückweg, Sitzungsmodell und Prozess-/Netzwerktopologie.
- Neu: ADRs unter `docs/adr/` im Format „Kontext / Entscheidung / Alternativen / Konsequenzen“:
  - CLIProxyAPI als eigener Prozess, die Kopie unter `third_party/` als eigenes Go-Modul
  - Zielstruktur des Repos (bestätigt, mit kleinen Änderungen)
  - Bibliotheken: Router, SQLite ohne cgo, YAML, JSON-Schema, Verschlüsselung, Frontend
  - JSON-Verarbeitung per Byte-Splicing und Determinismus-Regeln
  - Redactor-Port: was 1:1 übernommen und was neu entworfen wird
  - Platzhalter, Sitzungen und Rückweg
  - Kompression und Headroom-Port
  - Konfigurationsschema: Bootstrap-Konfiguration und versionierte Redactor-/Kompressionskonfiguration
  - Authentifizierung, Schlüssel und Geheimnisse
- Neu: `docs/adr/README.md` als Index der ADRs.
- Nach der Freigabe werden die Folge-Issues #2–#25 an die ADRs angepasst, wo die Entscheidungen von der ursprünglichen Planung abweichen.
- Kein Produktionscode, keine Kopie von Drittcode. Die Kopie von CLIProxyAPI kommt erst mit #3.

## Capabilities

### New Capabilities
Keine. Dieser Change liefert ausschließlich Dokumentation und ändert kein beobachtbares Systemverhalten. Die Anforderungen entstehen in den Specs der Folge-Changes ab #2. Deshalb ist `skip_specs: true` gesetzt.

### Modified Capabilities
Keine.

## Impact

- **Dateien:** `docs/ARCHITECTURE.md`, `docs/adr/*.md`
- **Folge-Issues:** #2–#25 werden nach der Freigabe überarbeitet. Absehbar betroffen:
  - #3: separater Prozess und Konfiguration von CLIProxyAPI
  - #5 und #6: Byte-Splicing und Header-Passthrough
  - #12 bis #14: Platzhalter, Sitzungen und Rückweg
  - #19 und #20: Umfang des Headroom-Ports
- **Abhängigkeiten:** Es werden noch keine eingeführt. Die ADRs legen die Bibliotheken fest, die ab #2 hinzukommen.
- **Gate:** Die Freigabe durch Niclas schließt #1 und entsperrt #2.

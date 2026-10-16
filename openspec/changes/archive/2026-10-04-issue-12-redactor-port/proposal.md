# Proposal

Issue: [#12 Redactor-Port nach Go mit Paritätstest](https://github.com/niclasedge/tollgate/issues/12)

Geht zurück auf GitHub-Issue #12. Die Abhängigkeit #2 ist geschlossen. Grundlage ist die
freigegebene Entscheidung [ADR-0005](../../../docs/adr/0005-redactor-port.md).

## Why

tollgate soll Geheimnisse ersetzen, bevor eine Anfrage das Haus verlässt. Die Erkennung gibt
es bereits im Rust-Redactor (`ref/hook-redactor`), aber als Hook-CLI mit sequenzieller
Ersetzung, Labelverlust und ohne Konfigurierbarkeit. Der Go-Port ist die Grundlage für die
Platzhalter mit Rückweg (#13), die Konfiguration (#15) und den Filter-Bereich (#16).

## What Changes

- Neues Paket `internal/redact`: die 19 portierten Regeln aus ADR-0005, die
  False-Positive-Filter, „erst alle Treffer sammeln, dann auflösen“ mit Priorität,
  Spannenlänge und Startposition, Secret-Gruppe statt Template und das neue
  `connection_urls`-Muster.
- Schlanke `Options` mit den sechs eingebauten Schaltern (`email`, `api_keys`, `jwt`,
  `connection_urls`, `ip_addresses`, `private_keys`) und ihren Standardwerten.
- Ersetzung in einem Durchgang über eine Platzhalter-Funktion; `Redact` liefert die
  unnummerierte Form `<redacted:TYP>`. Nummerierung und Rückweg folgen in #13.
- Korpus aller relevanten Rust-Testfälle als txtar unter `internal/redact/testdata/corpus/`,
  einschließlich red_team §7 und §9.
- Tabellengetriebene Tests, Determinismus-Test, Fuzz-Test und Paritätstest gegen das
  Rust-Binary.
- Herkunftseintrag in `third_party/NOTICE.md`, Nachträge zur Paritätsprüfung in ADR-0005.

## Capabilities

### New Capabilities

- `redaction`: Erkennung und Ersetzung von Geheimnissen im Go-Port.

### Modified Capabilities

Keine.

## Impact

Branch `issue-12/redactor-port`, gemeinsamer Workspace. Berührte Dateien:
`internal/redact/*.go`, `internal/redact/testdata/**`, `docs/adr/0005-redactor-port.md`,
`third_party/NOTICE.md` (nur angehängter Abschnitt), diese Change-Artefakte.

## Was ausdrücklich nicht

- Keine Pipeline-Verdrahtung, keine nummerierten Platzhalter, keine Rehydrate-Map (#13).
- Kein YAML-Laden, keine Wortliste, keine Allowlist, keine eigenen Regex (#15).
- Keine nicht portierten Rust-Regeln (Pfade, Username, Hostnamen, IBAN, Geburtsdatum, SVN)
  und keine CSV-, JSON- oder env-Modi.

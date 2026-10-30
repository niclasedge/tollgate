# Redactor · Filter

## Auf einen Blick

Der Redactor ist der Go-Port des Rust-Redactors (`internal/redact`) mit einer versionierten, über Frontend und YAML pflegbaren Konfiguration. Die Port-Regeln (Testfälle, Paritätstest) stehen im L0 `CLAUDE.md`, Abschnitt „Redactor-Port nach Go“. Hier: Konfiguration, Filtertypen, vertrauliche Wortlisten und das Beispiel-Schema.

## Konfiguration

Die gesamte Konfiguration ist über das Frontend pflegbar und als YAML im- und
exportierbar. Beide Wege führen zum selben Ergebnis.

- Die Konfiguration liegt versioniert in der DB. Jede Änderung erzeugt eine neue
  Version mit Zeitstempel und Diff, ein Rollback ist mit einem Klick möglich.
- YAML-Import: zuerst gegen `config/schema/` validieren und einen Dry-Run auf
  `testdata/fixtures/` laufen lassen. Erst nach Bestätigung aktivieren.
- Hot Reload ohne Neustart.
- Jede Sitzung hält ihre Konfigurationsversion fest. Neue Regeln gelten erst für neue
  Sitzungen, sonst ändern sich alte Nachrichten und der Prompt-Cache bricht.

Belege: ADR-0008 · ADR-0011

Wo: `internal/redactcfg/`, `config/schema/`, `testdata/fixtures/`, `openspec/specs/redaction-config/spec.md`

## Filtertypen

- Eingebaute Filter, einzeln schaltbar: E-Mail, API-Keys und Tokens, JWT,
  Verbindungs-URLs mit Passwort, IP-Adressen, private Keys.
- Eigene Regex-Filter mit Name, Platzhalter-Typ und Testbeispielen.
- Wortliste: bestimmte Wörter (Firmen, Kunden, Projekte), wahlweise mit
  Groß-/Kleinschreibung und nur ganze Wörter. Dazu eigene Listen für Kunden, Vornamen
  und Nachnamen mit den Platzhaltern `kunde`, `vorname`, `nachname` (ADR-0021).
- Allowlist für Werte, die nie ersetzt werden.

Belege: ADR-0005 · ADR-0021

Wo: `internal/redact/`, `openspec/specs/redaction/spec.md`

## Wortlisten

Die Wortlisten (allgemein, Kunden, Vornamen, Nachnamen) sind selbst vertraulich:
verschlüsselt speichern, im Frontend maskiert anzeigen, nie loggen. Ausnahme (neben dem
Aufdecken nach ADR-0018): „Bearbeiten“ lädt Kunden/Vornamen/Nachnamen auf Klick im
Klartext (ADR-0021). Der YAML-Export warnt vor dem Committen.

Belege: ADR-0009 · ADR-0018 · ADR-0021

Wo: `internal/cryptobox/`, `internal/redactcfg/`

## Beispiel-Schema

```yaml
version: 1
builtin:
  email: true
  api_keys: true
  jwt: true
  connection_urls: true
  ip_addresses: false
  private_keys: true
custom:
  - name: kundennummer
    pattern: 'KD-\d{6}'
    placeholder: customer_id
    examples: ["KD-123456"]
words:
  case_sensitive: false
  whole_word: true
  list: ["Projekt Falke", "Muster GmbH"]
allowlist: ["noreply@github.com"]
```

Das verbindliche Schema: `config/schema/tollgate-config.v1.schema.json`.

Belege: ADR-0008

Wo: `config/schema/tollgate-config.v1.schema.json`

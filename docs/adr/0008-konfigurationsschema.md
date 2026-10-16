# ADR-0008: Konfigurationsschema

- **Status:** Angenommen (Freigabe Niclas, 2026-10-04, PR #26)
- **Datum:** 2026-10-04
- **Issue:** #1

## Kontext

Der Auftrag verlangt:
- eine Redactor-Konfiguration, die im Frontend und als YAML pflegbar ist, mit identischem
  Ergebnis auf beiden Wegen
- Versionierung in der DB mit Diff und Rollback, Hot Reload
- Schema-Validierung und Dry-Run vor dem Aktivieren
- verschlüsselte Wortliste
- pro Sitzung eine gepinnte Version

Daneben braucht tollgate Startparameter: Adressen, Pfade, Schlüsseldateien. Diese dürfen
nicht über das Frontend änderbar sein. Wer das Frontend kompromittiert, darf weder Bindung
noch Upstream umbiegen können.

## Entscheidung

Es gibt zwei getrennte Ebenen.

### Ebene 1: Bootstrap-Konfiguration

- Datei `tollgate.yaml` (Pfad über `-config`). Jeder Wert ist per Env-Variable
  überschreibbar: `TOLLGATE_` plus Pfad in Großbuchstaben mit `_`.
- Nicht versioniert, nicht im Frontend, Änderung nur mit Neustart.

```yaml
listen: "100.101.102.103:8443"       # konkrete Tailnet-IP, keine Wildcard
upstream:
  url: "http://127.0.0.1:8317"       # CLIProxyAPI
  api_key_file: "/etc/tollgate/cpa.key"
management:
  url: "http://127.0.0.1:8317/v8/management"
  secret_file: "/etc/tollgate/cpa-management.secret"
db:
  path: "/var/lib/tollgate/tollgate.db"
crypto:
  key_file: "/var/lib/tollgate/master.key"   # wird beim ersten Start mit 0600 erzeugt
admin:
  token_file: "/etc/tollgate/admin.token"
log:
  level: info
```

- Geheimnisse stehen nie direkt in der Datei, nur Pfade zu Dateien mit Rechten 0600.
- **Startprüfung:** tollgate verweigert den Start, wenn eine der folgenden Bedingungen zutrifft:
  - `listen` ist eine Wildcard-Adresse (`0.0.0.0`, `::`, leer).
  - `listen` liegt außerhalb der Tailscale-Bereiche `100.64.0.0/10` bzw.
    `fd7a:115c:a1e0::/48`.
  - `upstream.url` zeigt nicht auf Loopback.

  Für lokale Entwicklung und Tests hebt das Flag `--dev-allow-any-listen` die
  Tailnet-Prüfung auf, nicht die Wildcard-Prüfung. Beim Start mit diesem Flag erscheint eine
  Warnung im Log.

### Ebene 2: Laufzeitkonfiguration (versioniert)

Ein Dokument mit einer Versionsnummer für Redact **und** Compress. Pinning, Rollback und
Dry-Run gelten so immer für den gesamten Transformationszustand. Das Beispiel aus `CLAUDE.md`
bleibt unverändert gültig. Neu ist nur der Abschnitt `compress:`.

```yaml
version: 1                    # Schema-Version, nicht die DB-Version
builtin:
  email: true
  api_keys: true
  jwt: true
  connection_urls: true
  ip_addresses: false
  private_keys: true
custom:
  - name: kundennummer        # eindeutig, [a-z][a-z0-9_]*
    pattern: 'KD-\d{6}'       # RE2-Syntax
    placeholder: customer_id  # Platzhalter-Typ laut ADR-0006
    examples: ["KD-123456"]   # Pflicht, mindestens 1, muss treffen
words:
  case_sensitive: false
  whole_word: true
  list: ["Projekt Falke", "Muster GmbH"]   # vertraulich, verschlüsselt gespeichert
allowlist: ["noreply@github.com"]
compress:                     # optional, fehlt er, gelten die Standardregeln
  min_bytes: 2048
  rules:
    - name: go_test
      match: ["go test"]
      strategy: test_log      # test_log | build_log | diff | search | passthrough
      max_errors: 10
      context_lines: 3
    - name: git_diff
      match: ["git diff", "git show"]
      strategy: diff
      max_files: 20
      max_hunks: 10
```

**Schema:**
- `config/schema/tollgate-config.v1.schema.json` (JSON Schema 2020-12) wird per `embed` ins
  Binary übernommen und unter `/api/config/schema` ans Frontend ausgeliefert. Dort validiert
  CodeMirror live gegen dasselbe Schema.
- `additionalProperties: false` auf allen Ebenen. Tippfehler werden abgelehnt, nicht
  ignoriert.

**Validierung in zwei Stufen:**
1. JSON-Schema, mit Pfad, Zeile und Spalte in der Fehlermeldung, z. B.
   `custom[0].placeholder (Zeile 14, Spalte 18): muss [a-z][a-z0-9_]{0,31} entsprechen`.
2. Semantik:
   - Regex kompiliert.
   - Jedes `example` trifft.
   - `name` ist eindeutig.
   - `placeholder` kollidiert nicht mit eingebauten Typen.
   - `match`-Präfixe sind nicht leer.

**Import (YAML oder Frontend-Formular, gleicher Codepfad):**
1. Parsen. YAML-Anker und Aliase sind erlaubt, mehrere Dokumente pro Datei nicht.
2. Validierung (beide Stufen).
3. Dry-Run auf `testdata/fixtures/`. Die Vorschau zeigt pro Fixture die Treffer und die
   Kompression, im Vergleich zur aktiven Version.
4. Bestätigung durch den Nutzer.
5. Neue Version in der Tabelle `config_versions`: `id`, `created_at`, `author` (Admin oder
   Import), `kommentar`, Inhalt mit verschlüsselter Wortliste, `diff` zur Vorgängerversion
   mit maskierter Wortliste.
6. Aktivieren: `atomic.Pointer` auf die kompilierte Konfiguration austauschen. Laufende
   Requests behalten ihren Zeiger, neue Sitzungen bekommen die neue Version.

**Export:**
- Deterministisch aus den Go-Structs mit fester Feldreihenfolge. Import und anschließender
  Export ergeben dieselbe Konfiguration (Round-Trip-Test).
- Die Wortliste wird im Klartext exportiert, sonst wäre der Export nicht importierbar.
  Die Datei beginnt mit dem Kommentar
  `# WARNUNG: enthält die vertrauliche Wortliste – nicht committen`. Das Frontend zeigt vor
  dem Download denselben Hinweis.
- Ein Export ohne Wortliste ist als Option verfügbar, mit `words.list: []`.

**Versionen und Rollback:**
- Versionen werden nie verändert oder gelöscht.
- Rollback legt eine **neue** Version mit dem Inhalt der Zielversion an. Die Historie bleibt
  linear und nachvollziehbar.

**Sitzungen:** Pinning siehe ADR-0006. Eine Konfigurationsänderung wirkt nur auf neue
Sitzungen.

### Kompilierte Form

Die Laufzeitkonfiguration wird beim Aktivieren einmal kompiliert:
- Regex
- Wortliste als Regex mit Alternativen, sortiert nach Länge absteigend, dann
  lexikografisch, damit das Ergebnis deterministisch ist
- Allowlist als Set
- Regeltabelle für Compress

Pro gepinnter Version liegt höchstens eine kompilierte Instanz im RAM. Ein LRU-Cache hält die
zuletzt genutzten Versionen.

## Alternativen

- **Alles in einer Datei, auch Bootstrap:** verworfen. Das Frontend könnte dann Bindung und
  Upstream ändern.
- **Getrennte Versionen für Redact und Compress:** verworfen. Zwei Pins pro Sitzung bringen
  mehr Komplexität und keinen Nutzen.
- **TOML wie im Rust-Redactor:** verworfen. Der Auftrag verlangt YAML, und CodeMirror plus
  JSON-Schema für YAML sind gut unterstützt.

## Konsequenzen

- **#4** legt die Tabellen `config_versions` und `sessions` an.
- **#15** setzt Schema, Validierung, Import, Export, Versionierung und Hot Reload um.
- **#16** baut Editor, Formular, Versionshistorie und Dry-Run-Vorschau.
- **#19** liest den Abschnitt `compress:`.

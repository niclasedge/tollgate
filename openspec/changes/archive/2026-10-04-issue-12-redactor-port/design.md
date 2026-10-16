# Design

Verbindlich ist [ADR-0005](../../../docs/adr/0005-redactor-port.md). Hier steht nur, wie der
Port die Entscheidung im Code umsetzt.

## API

```go
type Options struct{ Email, APIKeys, JWT, ConnectionURLs, IPAddresses, PrivateKeys bool }
func DefaultOptions() Options                 // alles an außer IPAddresses
type Finding struct{ Start, End int; Rule, Type string; Priority int }
type Redactor struct{ … }
func New(Options) *Redactor
func (*Redactor) Find(text string) []Finding  // aufgelöste Treffer, nach Start sortiert
func Replace(text string, fs []Finding, placeholder func(Finding) string) string
func (*Redactor) Redact(text string) string   // Replace mit <redacted:TYP>
```

`Start`/`End` sind Byte-Offsets der Secret-Gruppe im Originaltext. #13 ruft `Replace` mit
nummerierten Platzhaltern auf, #15 erweitert `Options` um Wortliste, Allowlist und eigene
Regex.

## Regeln

Die Regex-Texte sind aus `src/patterns.rs` übernommen. Wo eine Regel ein Template mit
Präfix hatte (`$1=…`, `$1$2 …`), wird nur der Wertteil als Capture-Gruppe markiert; bei
`auth_header`, `client_secret`, `azure_cred_guid` und `cert_thumbprint` kommt dafür eine
Klammer um den Wertteil hinzu. Das ändert nicht, was die Regex trifft. `conn_string` nutzt
das neue Muster aus ADR-0005.

| Regel | Schalter | Typ | Priorität |
|---|---|---|---|
| `conn_string` | `connection_urls` | `connection_password` | 30 |
| Anbieter-Keys, `jwt`, `pem_private`, `email`, `ipv4`, `ipv6` | je Schalter | je Typ | 20 |
| `ps_secret_field`, `azure_cred_guid`, `cert_thumbprint`, `client_secret`, `auth_header`, `bearer_token`, `generic_assign` | `api_keys` | `api_key` | 10 |

`conn_string` liegt über den übrigen spezifischen Regeln, weil `user:pass@host.tld` sonst
als E-Mail gelesen würde und das längere E-Mail-Match den Host verdeckte. Regeln, die nur
über einen Schlüsselnamen erkennen, gelten als generisch. Der Platz 100 und mehr ist für
Wortliste und eigene Regex (#15) frei.

## Auflösung

1. Jede aktive Regel läuft per `FindAllStringSubmatchIndex` über den Originaltext.
2. Ein Treffer fällt weg, wenn der False-Positive-Filter (Port von `is_false_positive`, mit
   16 Zeichen Kontext links und rechts wie in Rust) anschlägt oder die Secret-Spanne einen
   Platzhalter der ADR-0006-Grammatik überlappt.
3. Sortierung nach Priorität absteigend, Spannenlänge absteigend, Start aufsteigend,
   Regelreihenfolge. Gierig übernehmen, was keine schon übernommene Spanne überlappt.
4. Übernommene Treffer nach Start sortieren und in einem Durchgang ersetzen.

Alles hängt nur vom Text ab: keine Maps, keine Zeit, kein Zufall.

## Korpus

Ein Fall je Datei unter `testdata/corpus/<herkunft>/<name>.txtar`. Der Kommentar nennt die
Rust-Herkunft. Abschnitte: `input` (Pflicht), `must_contain`, `must_not_contain` (je Zeile
ein Teilstring), optional `want` (exakte Go-Ausgabe) und `deviation` (Name einer bewussten
Abweichung). Die Zeichenfolge `{{}}` wird beim Laden entfernt; sie trennt Schlüssel, die wie
echte aussehen, damit die Push-Protection von GitHub nicht anschlägt. Ein eigener kleiner
Parser liest das Format, damit `go.mod` unverändert bleibt.

## Paritätstest

Rust läuft mit `-t pattern --no-log --no-defaults --no-config --patterns <tmp>.json -` in
einem leeren temporären Verzeichnis, `REDACTOR_CONFIG_DIR` und `REDACTOR_IGNOREFILE_PATH`
zeigen auf nicht existierende Pfade. Die JSON-Datei enthält genau die portierten Regeln
mit Regex und Template aus `src/patterns.rs` des Referenz-Repos, in Rust-Reihenfolge. So
laufen die nicht portierten Regeln nicht mit, ohne dass der Korpus sie umgehen muss, und
die False-Positive-Filter greifen weiter, weil Rust sie über den Regelnamen auswählt.
Verglichen wird nach Normalisierung (`<redacted:…>` zu `<redacted>`, ein abschließendes
`\n` weg). Fälle mit `deviation` müssen sich unterscheiden, alle anderen gleichen.

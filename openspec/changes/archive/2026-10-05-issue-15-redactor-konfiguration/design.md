# Design

Verbindlich sind ADR-0008 (Schema, Import, Export, Versionen), ADR-0005 (Filtertypen),
ADR-0006 (Grammatik, Pinning) und ADR-0009 (Verschlüsselung). Hier steht, wie der Code die
Entscheidungen umsetzt.

## API

```go
// internal/redact
type Config struct {
    Builtin   Options
    Custom    []CustomRule // {Name, Pattern, Placeholder, Examples}
    Words     WordList     // {CaseSensitive, WholeWord, List}
    Allowlist []string
}
func Compile(Config) (*Redactor, error)        // Fehler: ConfigErrors{Path, Msg}
const TypeWord = "word"

// internal/redactcfg
type Sealer interface {                        // erfüllt von *cryptobox.Box (#28)
    Seal(id string, plain []byte) ([]byte, error)
    Unseal(id string, data []byte) ([]byte, error)
}
func Parse(yaml []byte) (*Document, error)      // Fehler: *ValidationError{Issues}
func Export(*Document, ExportMode) []byte
func Open(ctx, Store, Options) (*Manager, error)
func (*Manager) Active() *Compiled
func (*Manager) Get(ctx, version int64) (*Compiled, error)
func (*Manager) Redactor(ctx, version int64) (*redact.Redactor, error) // Resolver aus #13
func (*Manager) DryRun / Import / Activate / Rollback / Versions / Export / Test
func (*Manager) Register(*http.ServeMux)        // /api/config/…
```

`Compiled` ist unveränderlich: Versions-ID, `*redact.Redactor`, das Dokument intern.

## Kompilierte Form

- Eigene Regex: Regel `custom:<name>`, Priorität 100, ganzer Treffer, Typ = `placeholder`.
- Wortliste: eine Regel `words` je Wort, Priorität 110, Typ `word`. Die Wörter werden
  dedupliziert und nach Bytelänge absteigend, dann lexikografisch sortiert; diese Reihenfolge
  ist der letzte Gleichstand-Entscheider. Mehrwort-Einträge werden an Whitespace getrennt,
  einzeln mit `regexp.QuoteMeta` escaped und mit `[\s\p{Z}]+` verbunden, damit auch
  Zeilenumbrüche und geschützte Leerzeichen treffen. `case_sensitive: false` setzt `(?i)`.
  `whole_word` prüft links und rechts manuell mit `unicode.IsLetter`/`IsDigit`/`IsMark`,
  aber nur an Rändern, die selbst Buchstabe oder Ziffer sind.
- Abweichung von „eine Regex mit Alternativen“ (ADR-0008): Mit einer Alternation verdeckt
  ein längerer Kandidat, der an der Wortgrenze scheitert, einen kürzeren gültigen Treffer an
  derselben Stelle. Je Wort eine Regex, Neustart eine Rune nach einem verworfenen Treffer,
  und die Überlappungsauflösung aus #12 wählt den längsten. Das Ergebnis ist gleich
  deterministisch.
- Allowlist: Set, Vergleich exakt auf den Wert, der ersetzt würde (Secret-Gruppe), nach
  der Erkennung. Die eingebauten E-Mail-Ausnahmen bleiben aktiv.
- Hooks in bestehenden Dateien bleiben klein: Feld `find` im Typ `rule`, Feld `allow` im
  `Redactor`, `rl.matches(text)` statt direktem Regex-Aufruf in `Find`.

## Validierung

1. YAML parsen (`yaml.Node`). Anker und Aliase erlaubt, mehrere Dokumente und doppelte
   Schlüssel nicht. Merge-Schlüssel `<<` scheitern am Schema.
2. Node in JSON-Werte umwandeln und gegen das eingebettete Schema prüfen
   (`santhosh-tekuri/jsonschema/v6`, ADR-0003). Die Meldungen erzeugt tollgate selbst aus
   der Fehlerart, deutsch und ohne den geprüften Wert zu zitieren. Der Instanzpfad wird auf
   die Node zurückgeführt: `custom[0].placeholder (Zeile 14, Spalte 18): muss
   ^[a-z][a-z0-9_]{0,31}$ entsprechen`.
3. Semantik in `redact.Compile`: Regex kompiliert und trifft nicht den leeren Text, jedes
   Beispiel trifft, Namen eindeutig, Platzhalter-Typ nach ADR-0006 und nicht gleich einem
   eingebauten Typ oder `word`, Wörter nicht leer. Danach in `redactcfg`: Compress-Regeln
   eindeutig, `match`-Präfixe nicht leer.

Fehlermeldungen nennen bei der Wortliste nur den Index, nie den Eintrag.

## Export und Roundtrip

`Export` schreibt deterministisch aus den Go-Structs (feste Feldreihenfolge, zwei Leerzeichen
Einrückung, alle Schalter ausgeschrieben, leere Listen als `[]`, `compress` nur wenn
vorhanden). Modi:

- `ExportOmitWords` (Standard der API): `words.list: []`, Kopfkommentar „Wortliste nicht
  enthalten“.
- `ExportPlainWords` (`?words=plain`): Klartext, erste Zeile
  `# WARNUNG: enthält die vertrauliche Wortliste – nicht committen`, zusätzlich Header
  `X-Tollgate-Warning`.
- `ExportMaskedWords`: nur für Anzeige und Diff, nicht importierbar.

Roundtrip: `Parse(Export(d))` ergibt dasselbe Dokument, und `Export` darauf ist bytegleich.

## Versionen

- `content` = Export ohne Wortliste; `words_enc` = versiegeltes JSON-Array der Wörter,
  AAD `config_versions/<id>` (über `Seal(id)` im Schreib-Tx, #4). Ohne Wörter bleibt
  `words_enc` leer, ein Sealer ist dann nicht nötig.
- Ohne Sealer: Import mit Wörtern wird abgelehnt (`ErrNoSealer`), und eine gespeicherte
  Version mit `words_enc` lässt sich nicht laden. Klartext wird nie gespeichert.
- `diff`: zeilenweiser LCS-Diff der maskierten Exporte zur jeweils letzten Version, nur
  geänderte Zeilen mit `- `/`+ `, höchstens 64 KiB.
- Rollback legt eine neue Version mit dem Inhalt der Zielversion an und aktiviert sie.
- Import und Rollback laufen unter einem Mutex, damit der Diff zur tatsächlichen
  Vorgängerversion passt.
- Erster Start ohne aktive Version: Version 1 = Standardkonfiguration
  (`redact.DefaultOptions()`, keine eigenen Regeln), Autor `system`, aktiviert. So pinnt
  #13 jede Sitzung ab dem ersten Request auf eine echte Version.

## Hot Reload und Pinning

- `Activate`: Version kompilieren (oder aus dem Cache), `SetActiveConfigVersion`, dann
  `atomic.Pointer.Store`. Leser sehen entweder die alte oder die neue Version, nie einen
  Zwischenstand; laufende Requests behalten ihren Zeiger.
- `Get(version)` liefert jede gespeicherte Version, unabhängig von der aktiven. Ein LRU
  (8 Einträge, Mutex) hält kompilierte Versionen; Version 0 ist die eingebaute
  Konfiguration ohne Store-Zugriff.
- Das Pinning selbst (Tabelle `sessions`) macht `pipeline.SessionPinner` aus #13; die Stufe
  ruft dann `Manager.Redactor(ctx, pinnedVersion)`.

## Dry-Run

Fixtures `*.json` aus einem `fs.FS` (im Binary eingebettet über das Root-Paket
`tollgate`), Format wie die vorhandene Fixture (`{"body": {...}}`). Je Fixture laufen alle
Ziele von `jsonsplice.Scan` durch die aktive und die Kandidaten-Konfiguration; die Antwort
nennt pro Fixture die Treffer beider Seiten als Pfad, Offset, Länge, Typ und Regel, nie den
Wert.

## Admin-API

Alle Routen hinter `auth.RequireAdmin` (gleicher Mux wie Dashboard und Clients).

| Methode und Pfad | Zweck |
|---|---|
| `GET /api/config/schema` | eingebettetes JSON-Schema |
| `POST /api/config/dry-run` | `{yaml}` → Validierung und Dry-Run, 422 mit `issues` |
| `POST /api/config/import` | `{yaml, comment, activate}` → neue Version, optional aktiv |
| `GET /api/config/versions` | Liste mit `active`, Diff maskiert |
| `GET /api/config/versions/{id}` | Detail mit maskiertem YAML |
| `POST /api/config/versions/{id}/activate` | Hot Reload auf diese Version |
| `POST /api/config/versions/{id}/rollback` | neue Version mit Inhalt von `{id}`, aktiv |
| `GET /api/config/export?version=&words=plain` | YAML, Standard ohne Wortliste |
| `POST /api/config/test` | `{text, version?, yaml?}` → Treffer und redigierter Text |

Fehler kommen als `{"error": "...", "issues": [...]}`. Logs nennen nur Versions-IDs und
Fehlerarten, nie YAML, Wörter oder Testtexte.

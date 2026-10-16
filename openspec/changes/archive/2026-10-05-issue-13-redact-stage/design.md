# Design

Verbindlich ist [ADR-0006](../../../docs/adr/0006-platzhalter-sitzungen-rueckweg.md). Hier
steht, wie die Stufe die Entscheidung im Code umsetzt.

## API

```go
// internal/redact
const StageName = "redact"
const BuiltinConfig = "builtin-v1"            // Version 0: DefaultOptions()
type Resolver interface {
    Redactor(ctx context.Context, version int64) (*Redactor, error)
}
func Builtin() Resolver                       // ignoriert die Version, liefert builtin-v1
type Stage struct{ Resolver Resolver }
func NewStage(Resolver) *Stage                // implementiert pipeline.Stage
type Result struct {
    Replacements []jsonsplice.Replacement
    Placeholders map[string]string            // Platzhalter -> Klartext, nur RAM
    Matches      []pipeline.Match
    Diff         string
}
func Number(r *Redactor, targets []jsonsplice.Target) (Result, error)

// internal/pipeline
type Match struct{ Offset, Length int; Type, Rule string }
func (*Request) Report(diff string, matches []Match)  // Stufenbericht für den Trace
Request.Placeholders map[string]string                // `json:"-"`, für #14
type SessionPinner struct{ Store SessionStore }       // implementiert ConfigPinner
```

`Number` ist rein: gleiche Ziele und gleicher Redactor ergeben Byte für Byte dieselbe
Ausgabe. Maps dienen nur zum Nachschlagen, nie zum Iterieren.

## Nummerierung (ADR-0006)

1. Alle Ziele von `jsonsplice.Scan` in Dokumentreihenfolge. Das sind genau die Blöcke der
   Tabelle in ADR-0006 (User-String, `text` beider Rollen, `tool_result`, alle Strings in
   `tool_use.input`). Thinking, Signaturen, `system`, `tools`, `metadata`, Bilder,
   Dokumente und unbekannte Typen sind keine Ziele.
2. **Reservierung:** Jeder Text in Platzhalterform in diesen Zielen reserviert sein
   `(TYP, N)`. Thinking-Blöcke zählen bewusst nicht mit: Dort stehen die Platzhalter, die
   das Modell im vorigen Turn gesehen hat. Würden sie reservieren, bekäme dasselbe Geheimnis
   im nächsten Turn eine andere Nummer, und Präfix und Thinking passten nicht mehr zusammen.
3. Pro Ziel `Redactor.Find` auf dem dekodierten Wert. Pro Typ bekommt jeder neue Wert die
   kleinste freie Nummer größer als die zuletzt vergebene, reservierte Nummern werden
   übersprungen. Gleicher `(TYP, Wert)` ergibt denselben Platzhalter.
4. Mehr als 999999 Nummern eines Typs passen nicht in die Grammatik: Die Stufe bricht mit
   500 ab, statt ungefiltert weiterzuleiten.
5. Ziele ohne Treffer erzeugen keine Ersetzung und bleiben bytegleich. Ziele mit Treffern
   werden von `jsonsplice.Apply` mit dem festen Escaper neu geschrieben.

## Zuordnung nur im RAM, Abweichung vom Auftragswortlaut

AGENTS.md und der Auftrag sprechen von „Zuordnung im RAM mit TTL pro Sitzung“. ADR-0006
hat das nach der Architekturfreigabe ersetzt: Die Zuordnung entsteht **pro Anfrage** aus
der Anfrage selbst und lebt nur so lange wie der Request (`Request.Placeholders`). Das ist
strenger als eine Sitzungs-TTL (kürzere Lebensdauer, kein Zustand, kein Problem mit
Subagenten oder Neustarts) und trotzdem cachefest, weil der Präfix in jedem Turn dieselben
Nummern erzeugt. Die Sitzung pinnt nur die Konfigurationsversion.

## Stufenbericht und Trace

Die Stufe ruft `req.Report(diff, matches)`. Der Runner setzt den Bericht vor jeder Stufe
zurück und übernimmt ihn bei Erfolg in `StageTrace.Diff` und `StageTrace.Matches`; der Proxy
schreibt beides in die vorhandenen Store-Felder.

- **Treffer:** Offset und Länge beziehen sich auf den **Platzhalter im Ausgabe-Body** der
  Stufe, nicht auf das Geheimnis. So verrät auch die Länge nichts. Höchstens 1000 Treffer.
- **Diff:** eine Zeile pro Platzhalter, `Pfad: …Kontext<redacted:TYP_N>Kontext…`, Kontext
  höchstens 24 Bytes je Seite aus dem **redigierten** Wert, auf Runen-Grenzen gekürzt,
  JSON-escaped. Höchstens 32 KiB (Store-Grenze 64 KiB), danach eine Zeile `… N weitere`.
- Hashes und Tokens erfasst der Runner wie bisher über die redigierten Bytes.

## Konfigurationsversion

Bis #15 gibt es nur `builtin-v1` = `redact.DefaultOptions()`. Der `Resolver` bekommt die
gepinnte Version aus `Request.ConfigVersion`; `Builtin()` ignoriert sie. #15 liefert einen
Resolver, der die Version aus dem Store lädt und zwischenspeichert.

`pipeline.SessionPinner` fragt die aktive Version im Store ab. Ohne aktive Version gilt 0
(`builtin-v1`) ohne Schreibzugriff. Mit aktiver Version und Sitzungsschlüssel wird über
`Store.PinSession` gepinnt; die zuerst gespeicherte Version gewinnt. Ohne Sitzung gilt die
aktive Version.

## Schalter `redact.enabled`, Standard aus

Ohne Rückweg (#14) sieht Claude Code die Platzhalter in Antworten und `tool_use`-Eingaben.
Das Modell würde etwa `cat <redacted:api_key_1>` als Befehl ausführen lassen. Das ist für
den Alltag unbrauchbar. Deshalb ist die Stufe per Bootstrap schaltbar (`redact.enabled`,
`TOLLGATE_REDACT_ENABLED`) und standardmäßig aus. Eingeschaltet loggt tollgate beim Start
eine Warnung. Nach dem Merge von #14 wird der Standard in einem eigenen Issue umgestellt.
Alle Tests laufen mit eingeschalteter Stufe.

## Spike `cc_version`

Quelle: CLIProxyAPI am Pin (`internal/runtime/executor/claude_executor_cloaking.go`,
`computeFingerprint`). Der Fingerprint ist
`SHA-256("59cf53e54c78" + u[4] + u[7] + u[20] + version)[:3]`, `u` sind die UTF-16-Einheiten
des ersten nicht als Hinweis erkannten `text` der ersten User-Nachricht (fehlende Stellen
`'0'`).

Befund:
- Redact verändert den Fingerprint nur, wenn ein Treffer der ersten User-Nachricht vor
  UTF-16-Index 21 beginnt und sich dadurch eines der drei Zeichen ändert. In Claude Code
  steht dort meist der Prompt-Anfang; ein Geheimnis in den ersten 21 Zeichen ist selten.
- Ob Anthropic den Fingerprint prüft, lässt sich ohne echten Verkehr nicht belegen.
  CLIProxyAPI berechnet ihn nur neu, wenn der Billing-Block fehlt.

Entscheidung für #13: `system` bleibt unverändert (ADR-0006). Die Stufe prüft
deterministisch, ob sich die drei Zeichen in einem Text der ersten User-Nachricht ändern,
und schreibt dann als erste Diff-Zeile `note: cc_version fingerprint input changed`. So
lässt sich im Betrieb zählen, ob solche Anfragen auffallen. Erst mit Belegen wird über das
Entfernen des Billing-Blocks entschieden (eigenes ADR, eigenes Issue).

## Tests

- `stage_test.go`: Tabellen (Blocktabelle, Endpunkte, Nummerierung, Reservierung, Typen),
  Thinking und Signatur bytegleich, unbekannte Blocktypen und Felder bytegleich,
  Determinismus, Cache-Test über drei Turns (identische Präfixe, gleiche Nummern), Trace
  ohne Klartext, Fingerprint-Hinweis, Überlauf.
- `stage_fuzz_test.go`: `FuzzStage`: Redact, dann Rehydrate mit der Zuordnung, ergibt exakt
  die Ziele der Eingabe; zweiter Lauf bytegleich.
- `pipeline`: Bericht im Trace, Rücksetzen zwischen Stufen, `SessionPinner` mit echtem Store.
- `config`: Schalter aus Datei und Umgebung, ungültiger Wert.
- `server/redact_stage_test.go`: Leck-Test über Upstream, Store-Datei samt WAL und Logs.
- `cmd/tollgate/redact_wiring_test.go`: Schalter aus lässt den Body unverändert, an hängt die
  Stufe ein und warnt beim Start.

# Proposal

Issue: [#84 Redactor: client_secret-Grenzen und Vorfilter für Leistung](https://github.com/niclasedge/tollgate/issues/84)

Geht zurück auf den Port-Bericht aus #12 (geschlossen). Verbindlich ist
[ADR-0005](../../../docs/adr/0005-redactor-port.md) (Regex-Texte unverändert,
Secret-Gruppe, Paritätstest). Baut auf #83 auf (gleiche Funktion `Find`), ist fachlich aber
unabhängig.

## Why

1. `client_secret` greift den Wert mit `\S{8,}`. Bei `client_secret: "geheim…",` landen das
   schließende Anführungszeichen und das Komma in der Ersetzung, bei kompaktem JSON sogar die
   folgenden Felder (`"geheim…","id":1}`). Umkehrbar, aber das Modell sieht kaputtes JSON.
2. `Find` lässt alle 19 Regex über jeden Text laufen, auch wenn ein Schlüsselwort wie
   `client_secret` oder ein `@` gar nicht vorkommt. Gemessen: rund 38 ms je 100 KB
   Bash-Ausgabe (2,6 MB/s).

## What Changes

- `client_secret`: Folgt der Wert auf ein öffnendes `"` oder `'`, endet die Secret-Spanne am
  ersten nicht mit `\` maskierten gleichen Anführungszeichen. Anführungszeichen, Komma und
  Rest des Objekts bleiben stehen. Ein leerer Wert (`""`) ist kein Treffer. Unquotierte Werte
  und Werte ohne schließendes Anführungszeichen behalten die bisherige Spanne. Der Regex-Text
  bleibt unverändert.
- Vorfilter je eingebauter Regel: eine Liste von Literalen, von denen jeder Treffer der Regex
  mindestens eines enthält (`@` für `email`, `eyJ` für `jwt`, Schlüsselnamen für die
  generischen Regeln …). Fehlen alle im Text, läuft die Regex nicht. Für `(?i)`-Regeln wird
  der Text einmal gefaltet (A–Z, dazu U+017F und U+212A, die Go-`(?i)` auf `s`/`k` faltet).
  Eigene Regex und Wortliste laufen weiter immer.
- Tests: Vorfilter ändert über den ganzen Korpus nichts an den Treffern (Standard- und
  Korpus-Schalter), Fuzz-Test mit und ohne Vorfilter, Faltungs-Prämisse, Benchmark
  `BenchmarkFind`.
- Korpus: `patterns/client_secret_snake_case` und `patterns/ps_secret_client_secret`
  angepasst, neu `tollgate/client_secret_json`. ADR-0005 bekommt einen Nachtrag (#84).

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `redaction`: neue Anforderungen „client_secret endet am schließenden Anführungszeichen“ und
  „Vorfilter verliert keinen Treffer“.

## Impact

Branch `issue-84/client-secret-vorfilter`, gemeinsamer Workspace. Berührte Dateien:

- `internal/redact/rules.go` (Felder `need`, `fold`, `trim`; Literale je Regel;
  `trimQuoted`)
- `internal/redact/redact.go` (Feld `noPrefilter`, Typ `prefilter`, `foldASCII`, Aufruf in
  `Find`; eigene Hunks neben denen von #83)
- `internal/redact/redact_test.go` (`TestClientSecretBounds`)
- `internal/redact/prefilter_test.go`, `internal/redact/prefilter_bench_test.go` (neu)
- `internal/redact/testdata/corpus/patterns/client_secret_snake_case.txtar`,
  `internal/redact/testdata/corpus/patterns/ps_secret_client_secret.txtar`,
  `internal/redact/testdata/corpus/tollgate/client_secret_json.txtar` (neu)
- `docs/adr/0005-redactor-port.md` (Nachtrag #84)

Keine neue Abhängigkeit. Ausgaben ändern sich nur für quotierte `client_secret`-Werte.

## Was ausdrücklich nicht

- Kein Vorfilter für eigene Regex und Wortliste (Literale wären aus der Regex abzuleiten;
  eigenes Issue, falls nötig).
- Keine Fenster-Suche um Schlüsselwörter herum (Regex nur nahe dem Literal laufen lassen):
  Muster mit `\s*` und `[\s\S]` reichen über Zeilen, das bräuchte eine eigene Begründung.
- Keine Änderung an `generic_assign` oder `ps_secret_field` (beide schließen Anführungszeichen
  schon aus).

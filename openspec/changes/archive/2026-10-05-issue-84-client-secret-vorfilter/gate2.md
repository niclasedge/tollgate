# Gate 2

Branch `issue-84/client-secret-vorfilter`, gemeinsamer Workspace (geprüft wurde die
Kombination aller angewendeten Branches, einschließlich #83).

Rot mit Stubs (Felder `need`, `fold`, `trim` ohne Werte, `noPrefilter`, `prefilter.pass`
liefert immer wahr), `go test -race -run 'ClientSecret|Prefilter|Fold|Corpus' ./internal/redact/`:

- `TestClientSecretBounds`: z. B. `client_secret: "S3cretPwd!Long",` ergab
  `client_secret: "<redacted:api_key>` (Anführungszeichen und Komma ersetzt), kompaktes JSON
  verlor `"}` bzw. `","x":1}`, `client_secret="",abcdefgh` lieferte einen Treffer.
- `TestCorpus/patterns/client_secret_snake_case`, `TestCorpus/tollgate/client_secret_json`:
  Ausgabe ohne schließendes Anführungszeichen.
- `TestPrefilterSkipsRules`: für `hello world without anything` liefen alle 19 Regeln;
  keine eingebaute Regel hatte Literale.

Als Invarianten bereits grün (ohne Vorfilter trivial gleich) und nach der Umsetzung weiter
grün: `TestPrefilterKeepsCorpusFindings`, `TestPrefilterCaseFolding`, `FuzzPrefilter`,
`TestFoldOrbitsToASCII`. Ein erster Entwurf von `TestPrefilterCaseFolding` mit `ſecret=…`
scheiterte an der Prämisse (das `\b` vor `secret` verhindert den Treffer auch ohne Vorfilter)
und nutzt jetzt `paſſword=…`.

Nach der Umsetzung von `trimQuoted` wurde `patterns/ps_secret_client_secret` rot (gleiche
Spanne wie `ps_secret_field`, das schließende `'` bleibt stehen); der Fall ist an das neue,
gewollte Verhalten angepasst (design.md).

Grün nach der Umsetzung: `go test -race ./internal/redact/`, Paritätstest mit
`TOLLGATE_REDACTOR_REPO=$HOME/src/hook-redactor` (135 Korpusfälle,
25 bewusste Abweichungen), `go test -fuzz FuzzPrefilter -fuzztime 60s` (rund 457 000
Ausführungen) und `-fuzz 'FuzzRedact$' -fuzztime 40s` (rund 405 000), `make lint`,
`make test`, `openspec validate issue-84-client-secret-vorfilter --strict`.
Benchmark: design.md, Abschnitt Messung.

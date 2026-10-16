# Gate 2

Branch `issue-83/verdraengter-rest`, gemeinsamer Workspace (geprüft wurde die Kombination
aller angewendeten Branches).

Rot vor der Umsetzung (`go test -race -run 'Resolution|Corpus|Allowlist' ./internal/redact/`):

- `TestCorpus/tollgate/residual_of_displaced_generic`: Ausgabe
  `Authorization: Bearer <redacted:jwt> weiterer-text` enthält `weiterer-text`.
- `TestResolution/residual_of_displaced_generic`: Regeln `["jwt"]` statt
  `["jwt" "auth_header"]`.
- `TestResolution/residual_between_two_specifics`: Regeln `["email" "email"]` statt
  `["email" "auth_header" "email"]`, `mitte` im Klartext.

Bereits grün und als Schutz gegen zu viel Ersetzung gedacht: `residual without letter or
digit stays`, `key name of another rule stays` und der Allowlist-Fall in
`TestAllowlistWins`. Der erste Umsetzungsentwurf (nur Nicht-Secret-Teile anderer Kandidaten)
ließ genau diese Fälle und `jwt inside auth header` rot werden, weil `bearer_token` bei kurzen
Tokens (unter 16 Zeichen) nicht trifft; deshalb gelten `Bearer`, `Basic` und `Digest`
zusätzlich als Schema (design.md, ADR-0005-Nachtrag).

Grün nach der Umsetzung: `go test -race ./internal/redact/ ./internal/redactcfg/`,
Paritätstest mit `TOLLGATE_REDACTOR_REPO=$HOME/src/hook-redactor`
(134 Korpusfälle, 24 bewusste Abweichungen), `go test -fuzz FuzzRedact -fuzztime 40s`
(zwei Läufe ohne Befund), `make lint`, `make test`,
`openspec validate issue-83-verdraengter-rest --strict`.

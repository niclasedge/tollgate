# Proposal

Issue: [#83 Redactor: Rest eines verdrängten generischen Treffers nicht im Klartext lassen](https://github.com/niclasedge/tollgate/issues/83)

Geht zurück auf den Port-Bericht aus #12 (geschlossen). Verbindlich ist
[ADR-0005](../../../docs/adr/0005-redactor-port.md) („Erst alle Treffer sammeln, dann
auflösen“, Paritätstest).

## Why

Liegt ein spezifischer Treffer (z. B. JWT) in der Secret-Spanne eines generischen Treffers
(`auth_header`, `generic_assign` …), fällt der generische Treffer heute komplett weg. Was
von seinem Wert außerhalb des spezifischen Treffers liegt, bleibt im Klartext:
`Authorization: Bearer <jwt> weiterer-text` geht mit `weiterer-text` an Anthropic.

## What Changes

- Wird ein generischer Treffer (Priorität `prioGeneric`) wegen Überlappung verdrängt, werden
  die nicht abgedeckten Teile seiner Secret-Spanne als eigene Treffer gleicher Regel und
  gleichen Typs übernommen (Restspannen).
- Eine Restspanne wird an den Rändern um Whitespace gekürzt. Sie entfällt, wenn sie keinen
  Buchstaben und keine Ziffer enthält (z. B. ein schließendes `'`), wenn sie ganz im
  Nicht-Secret-Teil eines anderen Kandidaten liegt (Schema `Bearer` von `bearer_token`,
  Schlüsselname `password=` von `generic_assign`), wenn sie ein Schema `Bearer`, `Basic`
  oder `Digest` ist oder wenn ihr Wert in der Allowlist steht.
- Neuer Korpusfall `tollgate/residual_of_displaced_generic`, Unit-Tests in
  `TestResolution` und `TestAllowlistWins`.
- ADR-0005 bekommt einen Nachtrag (#83); die Beschreibung der bewussten Abweichung
  `overlap` in `parity_test.go` wird um die Restspannen ergänzt.

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `redaction`: Anforderung „Erst sammeln, dann auflösen“ um Restspannen ergänzt.

## Impact

Branch `issue-83/verdraengter-rest`, gemeinsamer Workspace. Berührte Dateien:

- `internal/redact/redact.go` (`Find`, neue Hilfsfunktion für Restspannen)
- `internal/redact/redact_test.go` (`TestResolution`, vier Fälle)
- `internal/redact/config_test.go` (`TestAllowlistWins`, Restspanne in der Allowlist)
- `internal/redact/parity_test.go` (Text von `knownDeviations["overlap"]`)
- `internal/redact/testdata/corpus/tollgate/residual_of_displaced_generic.txtar` (neu)
- `docs/adr/0005-redactor-port.md` (Nachtrag)

Keine neue Abhängigkeit, keine Migration. Platzhalter-Nummerierung und Rückweg bleiben
unverändert; es entstehen nur zusätzliche, nicht überlappende Treffer.

## Was ausdrücklich nicht

- Keine Restspannen für verdrängte spezifische Treffer (z. B. E-Mail unter `conn_string`):
  dort ist der Rest bewusst kein Geheimnis (`@host.tld`).
- Keine Änderung an Regex-Texten oder Prioritäten.
- client_secret-Grenzen und Vorfilter sind #84.

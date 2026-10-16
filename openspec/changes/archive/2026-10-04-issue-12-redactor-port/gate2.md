# Gate 2

Branch `issue-12/redactor-port`, gemeinsamer Workspace. Rot: mit einem Overlay, in dem
`Redactor.Find` keine Treffer liefert (`go test -overlay … ./internal/redact/`), schlagen
110 Tests und Untertests fehl, darunter `TestCorpus`, `TestSwitches`, `TestResolution` und
`TestParityWithRust`. Grün nach der Umsetzung: `go test -race ./internal/redact/...`
(Paritätstest gelaufen, 133 Fälle, 23 markierte Abweichungen), `make lint`, `make test`.

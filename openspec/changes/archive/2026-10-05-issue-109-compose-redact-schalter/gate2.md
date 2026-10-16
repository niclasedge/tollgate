# Gate 2
Branch `issue-109/compose-redact-schalter`, gemeinsamer Workspace.

## Roter Vorlauf
Vor der Umsetzung `go test -count=1 -run TestComposePassesRedactSwitch ./deploy/`: FAIL, `tollgate environment TOLLGATE_REDACT_ENABLED = "", want "${TOLLGATE_REDACT_ENABLED:-false}"`. Log lokal unter `/tmp/tollgate-issue109-red.log`.

## Validierung
- Nach Umsetzung `go test ./deploy/` grün.
- Hermetisch auf dem Branch-Inhalt: `make lint`, `make test`, `make build`, `openspec validate --all --strict`.

# Gate 2 — issue-75-cpa-egress

- Datum: 2026-10-05T07:20:19Z
- Person: niclasedge (Subagent im Auftrag des Koordinators)
- Branch: issue-75/cpa-egress (noch nicht angewendet)
- Pruefumfang: gemeinsamer Workspace aller angewendeten Branches; kein isolierter Branch-Nachweis
- Commit: a50c4398a05ab07f4f764b7f3cc04bcbad9e60cc
- Werkzeug: `just gate2` verweigert ohne angewendeten Branch; roter Lauf gezielt auf das neue Testpaket. Der volle `make test` war zu diesem Zeitpunkt zusaetzlich durch einen fremden roten Test (`internal/upstream/profile_test.go`, #29) rot und ist daher kein eigener Nachweis.

## Pruefungen

- bestanden — G2-1: Gate 1 liegt vor (gate1.md)
- bestanden — G2-2: die zugesagte Testdatei existiert
- bestanden — G2-3: die Testdatei ist neu gegenueber main
- bestanden — G2-4: der Testlauf schlaegt fehl (roter Test vor der Umsetzung)

## Belege

Befehl: `go test -race -count=1 ./deploy/cliproxyapi/` — Exit 1

```
--- FAIL: TestCPAEntrypointLocksEgressBeforeStart (0.00s)
        entrypoint_test.go:118: entrypoint missing: stat deploy/cliproxyapi/entrypoint.sh: no such file or directory
--- FAIL: TestCPAEntrypointFailsClosed (0.00s)
        entrypoint_test.go:157: entrypoint missing: stat deploy/cliproxyapi/entrypoint.sh: no such file or directory
--- FAIL: TestCPAEntrypointLegacyBackend (0.00s)
    entrypoint_test.go:174: entrypoint missing: stat deploy/cliproxyapi/entrypoint.sh: no such file or directory
--- FAIL: TestCPAComposeEgressCapabilities (0.00s)
    entrypoint_test.go:229: cpa-login cap_add = [], want [NET_ADMIN]
    entrypoint_test.go:232: cpa-login overrides the entrypoint and skips the egress rules
    entrypoint_test.go:229: cliproxyapi cap_add = [], want [NET_ADMIN]
    entrypoint_test.go:241: cpa-login command = []
    entrypoint_test.go:246: CPA image must install iptables
    entrypoint_test.go:250: CPA image must start through the egress entrypoint
    entrypoint_test.go:253: CPA image must not set USER; the entrypoint drops privileges after the rules
FAIL	github.com/niclasedge/tollgate/deploy/cliproxyapi	0.149s
```

# Gate 2 — issue-59-image-digests

- Datum: 2026-10-05T07:40:00Z
- Person: niclasedge (Subagent im Auftrag des Koordinators)
- Branch: issue-59/image-digests (noch nicht angewendet)
- Pruefumfang: gemeinsamer Workspace aller angewendeten Branches; kein isolierter Branch-Nachweis
- Commit: c64ca81c292880c4223735782b975ca75f44389f
- Werkzeug: `just gate2` verweigert ohne angewendeten Branch; roter Lauf gezielt auf das neue Testpaket.

## Pruefungen

- bestanden — G2-1: Gate 1 liegt vor (gate1.md)
- bestanden — G2-2: die zugesagte Testdatei existiert
- bestanden — G2-3: die Testdatei ist neu gegenueber main
- bestanden — G2-4: der Testlauf schlaegt fehl (roter Test vor der Umsetzung)

## Belege

Befehl: `go test -race -count=1 ./deploy/` — Exit 1. `TestUnpinnedFromRules` (10 Fälle der
Prüfregel) ist grün, rot ist nur die Prüfung der echten Dockerfiles.

```
--- FAIL: TestBaseImagesPinnedByDigest (0.02s)
    baseimages_test.go:93: deploy/cliproxyapi/Dockerfile line 5: golang:1.26-bookworm: base image needs name:tag@sha256:<index digest>
    baseimages_test.go:93: deploy/cliproxyapi/Dockerfile line 20: debian:bookworm-slim: base image needs name:tag@sha256:<index digest>
    baseimages_test.go:93: deploy/tollgate/Dockerfile line 4: node:22-bookworm-slim: base image needs name:tag@sha256:<index digest>
    baseimages_test.go:93: deploy/tollgate/Dockerfile line 14: golang:1.26-bookworm: base image needs name:tag@sha256:<index digest>
    baseimages_test.go:93: deploy/tollgate/Dockerfile line 24: debian:bookworm-slim: base image needs name:tag@sha256:<index digest>
FAIL
FAIL	github.com/niclasedge/tollgate/deploy	0.163s
FAIL
```

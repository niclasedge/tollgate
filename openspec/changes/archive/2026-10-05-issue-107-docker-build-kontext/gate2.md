# Gate 2 — issue-107-docker-build-kontext

- Datum: 2026-10-05T07:51:00Z
- Person: niclasedge (Subagent im Auftrag des Koordinators)
- Branch: issue-107/docker-build (noch nicht angewendet)
- Pruefumfang: gemeinsamer Workspace aller angewendeten Branches; kein isolierter Branch-Nachweis
- Commit: a5f76028a7bcd20bf1c4476380ac83d3fab674d6
- Werkzeug: `just gate2` ist lokal nicht installiert; roter Lauf gezielt auf das Testpaket.

## Pruefungen

- bestanden — G2-1: Gate 1 liegt vor (gate1.md)
- bestanden — G2-2: die zugesagte Testdatei existiert
- bestanden — G2-3: die Testdatei ist neu gegenueber main
- bestanden — G2-4: der Testlauf schlaegt fehl (roter Test vor der Umsetzung)

## Belege

Befehl: `go test -race -count=1 ./deploy/` — Exit 1. `TestMissingInputsRules` (Prüfregeln)
und `TestBaseImagesPinnedByDigest` sind grün; rot sind nur die Prüfungen des echten
Dockerfiles und der echten `.dockerignore`.

```
--- FAIL: TestTollgateImageBuildInputs (0.22s)
    buildcontext_test.go:286: deploy/tollgate/Dockerfile: config/schema/schema.go is needed by ./cmd/tollgate but not copied to /src/config/schema/schema.go or excluded by .dockerignore
    buildcontext_test.go:286: deploy/tollgate/Dockerfile: config/schema/tollgate-config.v1.schema.json is needed by ./cmd/tollgate but not copied to /src/config/schema/tollgate-config.v1.schema.json or excluded by .dockerignore
    buildcontext_test.go:286: deploy/tollgate/Dockerfile: fixtures.go is needed by ./cmd/tollgate but not copied to /src/fixtures.go or excluded by .dockerignore
    buildcontext_test.go:286: deploy/tollgate/Dockerfile: testdata/fixtures/claude-code-2.1.289-messages.json is needed by ./cmd/tollgate but not copied to /src/testdata/fixtures/claude-code-2.1.289-messages.json or excluded by .dockerignore
    buildcontext_test.go:286: deploy/tollgate/Dockerfile: testdata/fixtures/multi-turn-session.json is needed by ./cmd/tollgate but not copied to /src/testdata/fixtures/multi-turn-session.json or excluded by .dockerignore
    buildcontext_test.go:286: deploy/tollgate/Dockerfile: testdata/fixtures/redact-dry-run-messages.json is needed by ./cmd/tollgate but not copied to /src/testdata/fixtures/redact-dry-run-messages.json or excluded by .dockerignore
--- FAIL: TestDockerignoreReleasesOnlyFixtures (0.00s)
    buildcontext_test.go:308: testdata/fixtures/.gitkeep: excluded = true, want false
    buildcontext_test.go:308: testdata/fixtures/README.md: excluded = true, want false
    buildcontext_test.go:308: testdata/fixtures/claude-code-2.1.289-messages.json: excluded = true, want false
    buildcontext_test.go:308: testdata/fixtures/multi-turn-session.json: excluded = true, want false
    buildcontext_test.go:308: testdata/fixtures/redact-dry-run-messages.json: excluded = true, want false
FAIL
FAIL	github.com/niclasedge/tollgate/deploy	0.395s
FAIL
```

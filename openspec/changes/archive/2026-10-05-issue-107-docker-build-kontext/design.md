# Design

## Ursache

`cmd/tollgate` hängt von zwei Paketen außerhalb von `cmd/` und `internal/` ab: dem
Root-Paket `github.com/niclasedge/tollgate` (`fixtures.go`, Embed `testdata/fixtures/*.json`)
und `config/schema` (Embed `tollgate-config.v1.schema.json`). Die Build-Stage kopiert nur
`go.mod`, `go.sum`, `cmd`, `internal`, `web/*.go`, `web/placeholder.html` und `web/dist`.

## Fix

Drei `COPY`-Zeilen in der Build-Stage, vor `go build`:

```
COPY fixtures.go ./
COPY config ./config
COPY testdata/fixtures ./testdata/fixtures
```

`.dockerignore` behält `testdata` und ergänzt `!testdata/fixtures`. Die Fixtures sind
bereinigte, eingecheckte Testdaten (ADR-0008), also keine Secrets. `config/` enthält nur das
JSON-Schema und dessen Go-Paket.

## Test

`deploy/buildcontext_test.go`:

1. `go list -deps -json ./cmd/tollgate` im Repo-Root; nur Pakete mit `Module.Path ==
   github.com/niclasedge/tollgate`. Pflichtdateien sind `go.mod`, `go.sum` sowie `GoFiles`
   und `EmbedFiles` jedes Pakets, relativ zum Repo-Root.
2. Im Dockerfile die Stage mit `go build` finden, `WORKDIR` lesen und jede `COPY`-Zeile
   in Quelle(n) und Ziel zerlegen. Eine Kontext-Quelle (Datei, Verzeichnis oder Glob) legt
   eine Datei auf `Ziel/Rest`; `COPY --from=<stage>` legt Dateien unter seinem Ziel ab und
   gilt dort als Abdeckung (z. B. `web/dist` aus der `web`-Stage).
3. Eine Pflichtdatei gilt als abgedeckt, wenn eine Kontext-`COPY` sie aus demselben Pfad
   an denselben Pfad unter `WORKDIR` legt und `.dockerignore` sie nicht ausschließt, oder
   wenn ihr Pfad unter dem Ziel einer `COPY --from` liegt.
4. `.dockerignore`-Auswertung wie Docker: Muster in Reihenfolge, das letzte passende gewinnt,
   `!` nimmt aus; ein Muster trifft einen Pfad, wenn es ihn oder ein Elternverzeichnis per
   `path.Match` trifft. `**` wird nicht unterstützt und lässt den Test scheitern, damit die
   Prüfung nicht unbemerkt falsch wird.

Ein tabellengetriebener Test prüft die Regeln (Glob, Verzeichnis, Negation, `--from`,
falscher Zielpfad) unabhängig vom echten Dockerfile.

## Alternativen

- `moby/patternmatcher` für `.dockerignore`: exakt, aber eine neue Abhängigkeit nur für einen
  Test. Die eigene Auswertung reicht für die genutzte Syntax.
- `COPY . .` mit großzügiger `.dockerignore`: weniger Pflege, aber jede neue Datei im Repo
  landet im Build-Kontext. Explizite Quellen halten Secrets sicher draußen.

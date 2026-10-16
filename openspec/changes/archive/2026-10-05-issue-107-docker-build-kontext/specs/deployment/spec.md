## ADDED Requirements

### Requirement: Image-Build mit vollständigen Eingaben

`deploy/tollgate/Dockerfile` MUST jede Go- und Embed-Datei der Pakete dieses Moduls, von denen `./cmd/tollgate` abhängt, sowie `go.mod` und `go.sum` per `COPY` an ihren Repo-Pfad unter dem `WORKDIR` der Build-Stage legen, oder sie aus einer früheren Stage per `COPY --from` erhalten. `.dockerignore` MUST keine dieser Dateien ausschließen und SHALL von `testdata` nur `testdata/fixtures` freigeben.

#### Scenario: Fehlende Build-Eingabe wird erkannt
- **WHEN** `cmd/tollgate` ein Paket oder eine Embed-Datei nutzt, die keine `COPY`-Zeile der Build-Stage abdeckt oder die `.dockerignore` ausschließt
- **THEN** schlägt `make test` mit dem Pfad der fehlenden Datei fehl.

#### Scenario: Image baut aus dem Repo-Root
- **WHEN** `docker build -f deploy/tollgate/Dockerfile .` im Repo-Root läuft
- **THEN** läuft `go build` in der Build-Stage durch, und das Image entsteht.

#### Scenario: Nur Fixtures aus testdata
- **WHEN** der Build-Kontext aus `.dockerignore` entsteht
- **THEN** enthält er aus `testdata` nur `testdata/fixtures`.

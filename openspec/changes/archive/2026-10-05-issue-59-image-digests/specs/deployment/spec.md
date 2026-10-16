## ADDED Requirements

### Requirement: Basis-Images per Digest gepinnt

Jede `FROM`-Zeile eines Dockerfiles im Repository MUST ein externes Image mit Tag und Digest als `image:tag@sha256:<64 Hex-Zeichen>` nennen oder auf eine zuvor im selben Dockerfile mit `AS` benannte Stage verweisen. Der Digest SHALL der Digest des Multi-Arch-Index sein. `docs/DEPLOY.md` MUST beschreiben, wie die Digests ermittelt und aktualisiert werden.

#### Scenario: Floating Tag wird abgelehnt
- **WHEN** ein Dockerfile `FROM debian:bookworm-slim` ohne Digest enthält
- **THEN** schlägt `make test` mit Datei, Zeile und Image fehl.

#### Scenario: Gepinnte Images bestehen
- **WHEN** alle `FROM`-Zeilen in `deploy/tollgate/Dockerfile` und `deploy/cliproxyapi/Dockerfile` Tag und Digest tragen
- **THEN** besteht der Test, und beide Dateien wurden geprüft.

#### Scenario: Stage-Verweis bleibt erlaubt
- **WHEN** eine `FROM`-Zeile eine früher mit `AS` benannte Stage nennt
- **THEN** verlangt der Test dafür keinen Digest.

#### Scenario: Update nachvollziehbar
- **WHEN** ein Reviewer `docs/DEPLOY.md` liest
- **THEN** findet er den Befehl zum Ermitteln des Index-Digests und die Schritte bis zum Smoke-Test.

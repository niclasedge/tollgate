# issue-59-image-digests

Issue: [#59 Basis-Images per Digest pinnen (node, golang, debian)](https://github.com/niclasedge/tollgate/issues/59)

## Warum

Geht zurück auf GitHub-Issue #59 (Review von #57, PR #58). Abhängigkeit #57 ist geschlossen.
Die Build- und Laufzeit-Images in `deploy/tollgate/Dockerfile` und
`deploy/cliproxyapi/Dockerfile` (`node:22-bookworm-slim`, `golang:1.26-bookworm`,
`debian:bookworm-slim`) laufen mit Floating Tags. Ein Rebuild kann damit unbemerkt andere
Basis-Images ziehen, und zwei Builds desselben Commits sind nicht vergleichbar. Vor dem
Public-Schalten soll jedes Basis-Image reproduzierbar und prüfbar festgelegt sein.

## Was ändert sich

- Jede `FROM`-Zeile in beiden Dockerfiles nennt Tag und Multi-Arch-Index-Digest
  (`image:tag@sha256:<64 hex>`). Der Tag bleibt als lesbare Angabe stehen, maßgeblich ist der
  Digest.
- Ein Go-Test (`deploy/baseimages_test.go`) findet alle Dockerfiles im Repo und lässt jede
  `FROM`-Zeile ohne Digest scheitern; Verweise auf frühere Stages bleiben erlaubt.
- `docs/DEPLOY.md` beschreibt den Update-Ablauf der Basis-Images.

## Was ausdrücklich nicht

- Kein Wechsel der Tags oder Versionen (Node 22, Go 1.26, Debian bookworm bleiben).
- Kein Renovate/Dependabot: automatische PRs passen nicht zum Ablauf „ein Issue = ein
  Change“; der Update-Weg ist dokumentiert und von Hand.
- Keine Änderung am Verhalten der Images, am CPA-Entrypoint (ADR-0017), am CPA-Pin
  (ADR-0014) oder an `docker-compose.yml`.

## Berührte Dateien

- `deploy/tollgate/Dockerfile`, `deploy/cliproxyapi/Dockerfile`
- `deploy/baseimages_test.go` (neu)
- `docs/DEPLOY.md`
- `openspec/changes/issue-59-image-digests/**`

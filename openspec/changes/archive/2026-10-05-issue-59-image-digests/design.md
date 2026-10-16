# Design

## Format

`FROM <image>:<tag>@sha256:<digest> [AS <stage>]`. Docker nutzt bei Tag und Digest nur den
Digest; der Tag dokumentiert, wovon der Digest stammt, und macht den Update-Ablauf lesbar.
Gepinnt wird der Digest des Multi-Arch-Index (`application/vnd.oci.image.index.v1+json`),
nicht ein Plattform-Manifest. So baut dasselbe Dockerfile auf amd64 und arm64.

## Ermittelte Digests (2026-10-05)

Quelle: `docker buildx imagetools inspect <image>:<tag>`, Feld `Digest` des Index.

| Image | Digest |
|-------|--------|
| `node:22-bookworm-slim` | `sha256:43ac6c60b8f89723f746e8a92ce91abd5017e627ce1ddfe4238355d3a30b772c` |
| `golang:1.26-bookworm` | `sha256:a688600ca24f8a4d3ca77f95b0dd40704a9fc787c826660eb7ba0b641b8b175d` |
| `debian:bookworm-slim` | `sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251` |

## Test

`deploy/baseimages_test.go` läuft über das Repo (ohne `.git`, `ref/`, `node_modules/` und
die ungetrackte Alt-Kopie `third_party/cliproxyapi/`) und sammelt alle Dateien, deren Name
mit `Dockerfile` beginnt. Jede `FROM`-Zeile (auch mit `--platform=`) MUST entweder eine
zuvor mit `AS` benannte Stage nennen oder `name:tag@sha256:<64 hex>` sein. Der Test verlangt
außerdem, dass beide bekannten Dockerfiles gefunden werden, damit er nicht leer besteht.

## Update-Weg

Von Hand, dokumentiert in `docs/DEPLOY.md`: Digest per `docker buildx imagetools inspect`
lesen, in der `FROM`-Zeile ersetzen, Images bauen und `just smoke`. Gleiche Basis-Images in
beiden Dockerfiles (golang, debian) bekommen denselben Digest. Renovate oder Dependabot
wären möglich, erzeugen aber PRs außerhalb des Issue/Change-Ablaufs; das bleibt eine
spätere, eigene Entscheidung.

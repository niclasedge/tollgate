# Auftrag · Ursprung

## Auf einen Blick

Der ursprüngliche Auftrag für tollgate: ein nachvollziehbares, testbares Gateway für alle Claude-API-Aufrufe aus Claude Code, gebaut in Phasen. Hier stehen Ziel, Grundprinzip, die lokalen Referenzen (Redactor, Headroom, CLIProxyAPI) und die Phasen mit ihrer Abnahme.

## Ziel

Baue tollgate: ein nachvollziehbares, testbares Gateway für alle Claude-API-Aufrufe aus
Claude Code. Jede Anfrage läuft durch eine feste Pipeline aus Redaction, Kompression,
Messung und Weiterleitung. Jede Stufe ist im Frontend Schritt für Schritt nachvollziehbar.

Grundprinzip: **Alles passiert im Proxy.** Auf den Clients gibt es keine Hooks, kein rtk,
keine CLAUDE.md-Injektion und keine Wrapper. Ein Client setzt nur die URL und einen
Client-Key, tollgate macht den Rest.

Belege: ADR-0001 · ADR-0002

Wo: `docs/ARCHITECTURE.md`, `README.md`

## Referenzen

Redactor und Headroom sind **reine lokale Referenzen** für die Entwicklung. Sie liegen
als Symlinks unter `ref/` im Repo, `ref/` steht in `.gitignore` und wird nie committet:
keine Kopie, kein Submodul, kein Vendoring. Wohin die Symlinks zeigen: `ls -l ref/`.

- Redactor (Rust, eigenes Repo): `ref/hook-redactor`
  → wird **nach Go portiert** als Paket `internal/redact`. Das Rust-Repo dient nur als
  Referenz für Logik und Testfälle.
- Headroom (Python): `ref/headroom`
  → nur als Referenz. Relevante Kompressionsstrategien in Go portieren.
- CLIProxyAPI (Go): https://github.com/router-for-me/CLIProxyAPI
  → **keine Kopie im Repo.** Der Container-Build holt CPA per `go install` gepinnt auf
  einen Commit (`ARG CPA_VERSION` in `deploy/cliproxyapi/Dockerfile`, die einzige Stelle).
  Commit, Lizenz und Update-Ablauf stehen in `deploy/cliproxyapi/UPSTREAM.md` (ADR-0014).

Belege: ADR-0005 · ADR-0007 · ADR-0014

Wo: `internal/redact/`, `internal/compress/`, `deploy/cliproxyapi/Dockerfile`, `deploy/cliproxyapi/UPSTREAM.md`, `third_party/NOTICE.md`

## Phasen

Gebaut wird in Phasen. Jede Phase endet mit grünen Tests und einem kurzen Bericht: was
gebaut wurde, wie es getestet ist, was offen ist. Phasen sind Milestones
(`gh api repos/niclasedge/tollgate/milestones`).

1. **Durchreichen und messen:** Proxy, Store, Client-Keys und Frontend mit Live-Liste,
   Kosten und Clients.
2. **Redactor:** Go-Port mit Paritätstest, Hin- und Rückweg mit Streaming,
   Filter-Bereich mit YAML-Import und -Export, Detailansicht mit Diffs.
3. **Kompression:** Regeln für die Befehle mit den meisten Tokens laut Log aus
   Phase 1, plus Headroom-Port und Regel-Playground.
4. **Ausbau:** Budget-Alarm, OpenTelemetry, Feinschliff am Frontend.

Abnahme jeder Phase: Claude Code arbeitet normal über tollgate, nur mit gesetzter URL
und Client-Key. Alle Tests sind grün, und die Cache-Trefferquote ist nicht schlechter
als ohne tollgate.

Wo: `docs/phasen/`, `docs/token-analyse.md`

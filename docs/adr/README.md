# Architecture Decision Records

Jede tragende Entscheidung für tollgate steht in einem ADR. Ein ADR wird nicht nachträglich
umgeschrieben. Ändert sich eine Entscheidung, gibt es ein neues ADR, das das alte ersetzt
(Status `Ersetzt durch ADR-NNNN`).

Übergeordnet: [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md).

## Index

| Nr. | Titel | Status |
|---|---|---|
| [0001](0001-cliproxyapi-als-eigener-prozess.md) | CLIProxyAPI als eigener Prozess | Teilweise abgelöst durch 0014 |
| [0002](0002-zielstruktur.md) | Zielstruktur des Repos | Teilweise abgelöst durch 0014 |
| [0003](0003-bibliotheken.md) | Bibliotheken | Angenommen |
| [0004](0004-json-byte-splicing-und-determinismus.md) | JSON per Byte-Splicing und Determinismus | Angenommen |
| [0005](0005-redactor-port.md) | Redactor-Port nach Go | Angenommen |
| [0006](0006-platzhalter-sitzungen-rueckweg.md) | Platzhalter, Sitzungen und Rückweg | Angenommen |
| [0007](0007-kompression-und-headroom-port.md) | Kompression und Headroom-Port | Angenommen |
| [0008](0008-konfigurationsschema.md) | Konfigurationsschema | Angenommen |
| [0009](0009-authentifizierung-und-geheimnisse.md) | Authentifizierung, Schlüssel und Geheimnisse | Angenommen |
| [0010](0010-gitbutler-workspace.md) | Paralleles Arbeiten im gemeinsamen GitButler-Workspace | Angenommen |
| [0011](0011-sitzungsschluessel-trennbyte.md) | Sitzungsschlüssel mit Trennbyte | Angenommen |
| [0012](0012-container-deployment-tailscale-sidecar.md) | Container-Deployment mit Tailscale-Sidecar und `just setup` | Teilweise abgelöst durch 0013 |
| [0013](0013-veroeffentlichung-auf-host-tailnet-ip.md) | Veröffentlichung auf der Tailnet-IP des Hosts statt Tailscale-Sidecar | Angenommen |
| [0014](0014-cliproxyapi-gepinnt-ohne-kopie.md) | CLIProxyAPI gepinnt aus dem Upstream statt als Kopie | Angenommen |
| [0015](0015-account-tresor-und-wechsel.md) | Account-Tresor und manueller Wechsel | Angenommen |
| [0016](0016-cpa-refresh-barriere.md) | Kleine CPA-Erweiterung als Refresh-Barriere | Angenommen |
| [0017](0017-cpa-egress-nur-anthropic.md) | Egress des CPA-Namespaces nur zu Anthropic | Angenommen |
| [0019](0019-alarme-spool-notifier.md) | Alarme über Spool und Notifier-Container | Angenommen |
| [0020](0020-cpa-katalog-overlay.md) | Katalog-Overlay für Modelle, die der CPA-Pin nicht kennt | Angenommen |
| [0021](0021-namenslisten-klartext-bearbeiten.md) | Namenslisten, Klartext nur zum Bearbeiten | Angenommen |

## Vorlage

```markdown
# ADR-NNNN: Titel

- **Status:** Vorgeschlagen | Angenommen | Ersetzt durch ADR-NNNN
- **Datum:** JJJJ-MM-TT
- **Issue:** #N

## Kontext
Was ist das Problem, welche Kräfte wirken, welche Fakten aus den Quellen sind relevant?

## Entscheidung
Was wird getan? Präzise genug, dass ein Folge-Issue daraus Specs ableiten kann.

## Alternativen
Welche Optionen wurden verworfen, und warum?

## Konsequenzen
Was wird dadurch leichter, was schwerer? Welche Folge-Issues sind betroffen?

## Quellen
Pfade in den Referenz-Repos mit Commit-Hash oder Version.
```

## Referenzstände

Alle Quellenangaben in den ADRs beziehen sich auf diese Stände:

| Quelle | Stand | Lizenz |
|---|---|---|
| CLIProxyAPI (`github.com/router-for-me/CLIProxyAPI`) | Commit `8ef43e4df3b216a42493105d31c2873b69191473`, Modul `/v8` | MIT |
| hook-redactor (lokal, `ref/hook-redactor`) | v0.8.1 | MIT |
| Headroom (lokal, `ref/headroom`) | Arbeitsstand 2026-10-04 | Apache-2.0 |

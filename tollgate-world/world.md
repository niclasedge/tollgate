# tollgate-world

## Identitaet

tollgate ist ein Gateway in Go für alle Claude-API-Aufrufe aus Claude Code: Redaction, Kompression, Messung und Weiterleitung über CLIProxyAPI, mit einem eingebetteten Svelte-Frontend, das jede Stufe nachvollziehbar macht. Du arbeitest im Repo niclasedge/tollgate.

Diese World beschreibt, wo was liegt und welche Regeln gelten. Zahlen, Status und Inventare stehen hier nicht; die Coverage-Map nennt die Befehle, die sie ableiten.

## Routen

- Auftrag → definiert → Pipeline
- Pipeline → ruft → Redactor
- Redactor → wird gepflegt in → Frontend
- Pipeline → zeigt sich in → Frontend
- Tests → prüft → Pipeline
- Tests → prüft → Redactor
- Zusammenarbeit → liefert → Tests

## Coverage-Map

Eine Zeile je Bereich. Nur den Abschnitt hinter dem Anker lesen, dann tiefer, wenn die Aufgabe es verlangt.

| Dimension | Exakte Adresse | Was drin steht | Lesen, wenn die Aufgabe verlangt … |
|---|---|---|---|
| Auftrag | `tollgate-world/auftrag.md#auf-einen-blick` | Ziel, Referenzen (`ref/`, CLIProxyAPI-Pin), Phasen und Abnahme | Verstehen, warum tollgate so gebaut ist, oder Redactor/Headroom/CPA-Code portieren. |
| Pipeline | `tollgate-world/pipeline.md#auf-einen-blick` | Datenfluss, Stufen, Technik, Nachvollziehbarkeit | Eine Pipeline-Stufe, den Proxy, Upstream oder Traces ändern. |
| Redactor | `tollgate-world/redactor.md#auf-einen-blick` | Konfiguration, Filtertypen, Wortlisten, Beispiel-Schema | Filter, Wortlisten, YAML-Import/-Export oder Konfigurationsversionen ändern. |
| Frontend | `tollgate-world/frontend.md#auf-einen-blick` | Technik, Funktionen, Design, Client-Einrichtung | Eine Ansicht, das Design oder die Client-Keys ändern. |
| Tests | `tollgate-world/test.md#auf-einen-blick` | Testarten, Standards | Tests schreiben oder auswählen. |
| Zusammenarbeit | `tollgate-world/zusammenarbeit.md#auf-einen-blick` | OpenSpec-Ablauf, GitButler, Subagenten | Ein Issue beginnen, committen oder Subagenten einsetzen. |
| Architektur | `docs/ARCHITECTURE.md` · `docs/adr/README.md` | Entscheidungen und ihre Begründung | Eine Entscheidung verstehen oder ändern. |
| Build | `tollgate-world/build.md#auf-einen-blick` | Commands and source locations | Run build checks. |
| Mission (live) | `gh issue list -R niclasedge/tollgate --state open` · `openspec list` | offene Arbeit und laufende Changes; nicht Teil der World | Wissen, was als Nächstes ansteht. |
| Zustand (live) | `just status` · `git log --oneline` · `but status` | Stack, Historie, Workspace | Zustand brauchen; nie aus einer Datei abschreiben. |
| Skills | `just --list` · `make` (Targets im `Makefile`) · `/opsx:*` | Betrieb, Build, Tests, OpenSpec | Einen Ablauf ausführen statt improvisieren. |
| Grenzen | `tollgate-world/boundaries.md` | was die World nicht entscheidet | Eine Architektur-, Deploy- oder Veröffentlichungsentscheidung treffen. |

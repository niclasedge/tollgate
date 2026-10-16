# Design

## Context

Siehe `proposal.md`. Grundlage sind die am 2026-10-04 mit Niclas geklärten Entscheidungen: Ops-lab-Werkzeug kommt aus einem eigenen Repo, `AGENTS.md` ist führend, Produkt- und Prozessregeln werden versioniert, die GitButler-Recherche wird zum ADR.

## Decisions

- **D1 Pauschales Ignorieren statt Namensliste:** `.claude/` und `.agents/` werden vollständig ignoriert. Ein neues Werkzeug kann so nicht versehentlich committet werden. Preis: Die OpenSpec-Skills entstehen lokal per `openspec init`.
- **D2 `AGENTS.md` führend, `CLAUDE.md` als Import:** Codex liest `AGENTS.md`, Claude Code löst `@AGENTS.md` in `CLAUDE.md` auf. Eine Quelle, kein Auseinanderlaufen.
- **D3 `docs/ops-lab.md` ohne Werkzeug-Interna:** Die Datei beschreibt, was der Ablauf leisten muss (Workspace, Gate-Kette, Veröffentlichung), nicht wie die externen Werkzeuge bedient werden. Verweise auf die geschlossenen Issues #31–#33 entfallen.
- **D4 `docs/standards.md` aus dem gesicherten Stand von #30 übernommen**, einzige Änderung: „AGENTS.md“ statt „CLAUDE.md“ als maßgebliche Regeldatei (Folge aus D2).
- **D5 ADR-0010** fasst die GitButler-Recherche als Entscheidung mit Alternativen zusammen; die Rohrecherche wird nicht versioniert.

## Risks

- `.claude/` enthält lokal auch `settings.json` und alte Worktrees; beides bleibt lokal und wird nicht mehr versioniert. Gewollt.

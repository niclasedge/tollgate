# Proposal

Issue: [#34 Repo-Hygiene: Agent-Werkzeug ignorieren, AGENTS.md führend, Standards und ADR 0010](https://github.com/niclasedge/tollgate/issues/34)

## Why

tollgate wird später öffentlich. Agent-Werkzeug (Ops-lab-Kopie, von `openspec init` erzeugte Skills) gehört nicht ins Produkt-Repo: Herkunft und Lizenz der Ops-lab-Kopie sind ungeklärt, und Werkzeug-Fehler würden in jeder Kopie gepflegt. Gleichzeitig fehlen werkzeugneutrale, versionierte Regeln für Claude Code und Codex sowie die begründete Entscheidung für parallele Arbeit im gemeinsamen GitButler-Workspace. Der Ansatz aus #30 (Ops-lab-Kopie im Repo) ist verworfen.

## What Changes

- `.gitignore` schließt `.claude/`, `.agents/`, `tools/`, `tests/*.mjs`, `justfile`, `docs/*SESSION*.md` und `__pycache__/` aus.
- Die von `openspec init` erzeugten Dateien `.claude/skills/openspec-*` und `.claude/commands/opsx/*` werden aus dem Repo entfernt.
- `AGENTS.md` wird führende Regeldatei: bisheriger Auftrag aus `CLAUDE.md` plus GitButler- und Subagenten-Regeln. `CLAUDE.md` enthält nur `@AGENTS.md`.
- Neu: `docs/standards.md` (Produkt- und Prozessstandards), `docs/ops-lab.md` (agentischer Ablauf ohne Werkzeug-Interna), `docs/AGENT_SETUP.md` (lokale Einrichtung).
- Neu: ADR-0010 „Paralleles Arbeiten im gemeinsamen GitButler-Workspace“, Eintrag im ADR-Index.
- Kein Produktionscode, keine Änderung an Build, Tests oder CI.

## Capabilities

### New Capabilities
Keine. Der Change betrifft nur Dokumentation und Repo-Hygiene und ändert kein beobachtbares Systemverhalten; deshalb `skip_specs: true`.

### Modified Capabilities
Keine.

## Impact

- **Dateien:** `.gitignore`, `AGENTS.md`, `CLAUDE.md`, `docs/standards.md`, `docs/ops-lab.md`, `docs/AGENT_SETUP.md`, `docs/adr/0010-gitbutler-workspace.md`, `docs/adr/README.md`, `docs/ARCHITECTURE.md` (Verweise auf die Regeldatei), entfernt: `.claude/skills/openspec-*/SKILL.md`, `.claude/commands/opsx/*.md`.
- **Folge-Issues:** #2 wird auf diesen Change gestapelt (Depends on #34).
- **Lokal:** Wer das Repo klont, führt einmal die Schritte aus `docs/AGENT_SETUP.md` aus.

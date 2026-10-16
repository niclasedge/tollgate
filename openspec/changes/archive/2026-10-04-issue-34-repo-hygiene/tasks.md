# Tasks

## 1. Repo-Hygiene

- [x] 1.1 `.gitignore` um Agent-Werkzeug ergänzen. Prüfen: `git check-ignore .claude/x .agents/x tools/x tests/x.mjs justfile docs/SESSION_X.md` meldet alle Pfade.
- [x] 1.2 `.claude/skills/openspec-*` und `.claude/commands/opsx/*` aus dem Repo entfernen. Prüfen: `git ls-files .claude .agents` ist leer.

## 2. Regeln

- [x] 2.1 `AGENTS.md` als führende Regeldatei anlegen, `CLAUDE.md` auf `@AGENTS.md` reduzieren. Prüfen: Der Auftrag aus dem bisherigen `CLAUDE.md` ist vollständig in `AGENTS.md` enthalten (`diff` zeigt nur Ergänzungen).
- [x] 2.2 `docs/standards.md` und bereinigte `docs/ops-lab.md` aufnehmen. Prüfen: `grep -E '#3[123]|projekt-test|just (gate|lage|messen)' docs/ops-lab.md` ist leer.
- [x] 2.3 `docs/AGENT_SETUP.md` mit `openspec init`, `but skill install` und Ops-lab-Hinweis. Prüfen: Alle relativen Links in `AGENTS.md` und `docs/` zeigen auf existierende Dateien.

## 3. Entscheidung

- [x] 3.1 ADR-0010 schreiben und im Index eintragen. Prüfen: Index verlinkt `0010-gitbutler-workspace.md`.

## 4. Abschluss

- [x] 4.1 `openspec validate issue-34-repo-hygiene --strict` grün; keine Secrets oder internen Namen (`grep -rniE '<employer-name>' AGENTS.md docs/` leer).
- [x] 4.2 PR mit `Closes #34` (#35), gemergt, danach archiviert.

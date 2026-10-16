# Agent-Werkzeuge lokal einrichten

Agent-Werkzeug liegt nicht im Repo, `.gitignore` schließt `.claude/`, `.agents/`,
`tools/`, `tests/*.mjs` und `docs/*SESSION*.md` aus. Das `justfile` ist versioniert (Betrieb, ADR-0012) und bindet lokale Rezepte per `import? 'tools/ops.just'` ein; ops-lab-Rezepte gehören dorthin, nicht ins `justfile`. Wer mit Claude Code
oder Codex an tollgate arbeitet, richtet es einmal lokal ein. Die Regeln selbst stehen in
[`AGENTS.md`](../AGENTS.md), der Ablauf in [`ops-lab.md`](ops-lab.md).

## OpenSpec

```sh
pnpm add -g @fission-ai/openspec
openspec init --tools claude,codex
```

Das erzeugt die Skills und Commands (`/opsx:propose`, `/opsx:apply`, `/opsx:archive` …)
unter `.claude/` bzw. `.agents/`. `openspec/` mit Specs und Changes ist Projektinhalt und
bleibt versioniert. Nach OpenSpec-Updates: `openspec update`.

## GitButler

```sh
but skill install --global
```

Installiert das GitButler-Skill im Home-Verzeichnis. `AGENTS.md` verlangt, dass alle
Schreiboperationen über `but` laufen.

## Ops-lab

Leitstand, Change-Arbeiter, PR-Reviewer, Prozessschmiede und die Gate-Werkzeuge kommen
aus einem eigenen Repo bzw. als Claude-Code-Plugin. Sie werden nach dessen Anleitung
lokal installiert. tollgate erwartet davon nur den Ablauf aus [`ops-lab.md`](ops-lab.md)
und ruft als Projektprüfung `make test` auf.

## Session-Übergaben

Übergaben laufen über Kommentare im jeweiligen GitHub-Issue. Lokale Notizen unter
`docs/*SESSION*.md` sind erlaubt, werden aber nie committet.

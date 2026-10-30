# Zusammenarbeit · Werkstatt

## Auf einen Blick

Mehrere Agenten (Claude Code, Codex, Subagenten) arbeiten parallel im selben GitButler-Workspace, jeder auf eigenem Branch, ein Issue je OpenSpec-Change. Hier: der OpenSpec-Ablauf, die GitButler-Regeln und die Regeln für Subagenten. Der übergeordnete Ablauf mit Gates und Veröffentlichung steht in `docs/ops-lab.md`.

## Tracking und OpenSpec

- Abhängigkeiten sind als GitHub-Dependencies gesetzt. Ein Issue wird erst begonnen, wenn
  alle Abhängigkeiten geschlossen sind.
- Phasen sind Milestones (siehe `auftrag.md#phasen`).
- **Ein Issue = ein OpenSpec-Change.** Ablauf:
  1. Change anlegen: `/opsx:propose`, Change-ID `issue-<N>-<slug>`, Issue im
     `proposal.md` verlinken.
  2. Proposal, Specs und Tasks ausarbeiten, `openspec validate` muss grün sein.
  3. Umsetzen (`/opsx:apply`), Tests grün.
  4. PR mit `Closes #<N>`, nach dem Merge Change archivieren (`/opsx:archive`).
- Neue Erkenntnisse während der Arbeit werden als neues Issue erfasst, nicht im
  laufenden Change mitgezogen.

Laufende Changes: `openspec list`.

Belege: ADR-0010

Wo: `openspec/changes/`, `openspec/specs/`, `docs/ops-lab.md`, `docs/AGENT_SETUP.md`

## GitButler

- Use GitButler (`but`) for version-control inspection and write operations, including status, diffs, branching, committing, pushing, and history edits.
- Assume multiple agents may be working in this repository. Do not move, amend, squash, discard, commit, push, or otherwise modify another agent's work unless the user asks.
- For commit just/only/specific changes on a new branch (selected-change requests), use the two-command fast path from the GitButler skill: `but diff`, then `but commit -b <branch> -m "message" <id> <id>`.
- For that fast path, after the commit succeeds, stop and summarize; do not run separate branch, staging, status, or diff commands unless the commit output is missing information you need.
- Use the installed GitButler skill for command recipes and syntax before guessing flags, using `--help`, or translating Git habits directly.
- Mutation commands report their result without appending workspace status. Add `--status-after` only when the next step needs resulting workspace IDs or details; otherwise do not rerun status or diff to verify success.
- Use a dedicated GitButler branch for each agent session and each writing subagent, following the local collaboration rules below. Commit only changes that belong to that agent.
- Do not push or open pull requests unless the user asks.
- Keep commit messages and pull request descriptions succinct: explain what changed, why it changed, and any important decision.

Amend local fixes into the right commits:

- For small cleanup or follow-up fixes, amend an unpublished local commit when the change clearly belongs with that commit's intent.
- Do not create tiny fixup commits unless the user asks.
- Use GitButler to move the relevant changes into the commit where they belong.
- Ask before rewriting pushed, reviewed, shared, or ambiguous history.

Split unrelated changes into separate commits:

- If one file contains unrelated changes, split them by hunk instead of committing the whole file.
- Keep tests with the behavior they verify.
- Split generated output, docs-only edits, or mechanical cleanup into separate commits when each commit remains coherent on its own.
- If the split is ambiguous, summarize the options before committing.

Belege: ADR-0010

Wo: `docs/ops-lab.md`, `docs/AGENT_SETUP.md`

## Subagenten

- Lokale Subagenten arbeiten im gemeinsamen GitButler-Workspace dieses Repos auf
  eigenen, parallel angewendeten GitButler-Branches. Keine neuen Git-Worktrees,
  separaten Checkouts oder Worktree-Isolation beim Start eines Subagenten verwenden.
  Diese Repo-Regel gilt auch, wenn ein allgemeiner Skill Worktrees empfiehlt.
- Der koordinierende Agent weist jedem schreibenden Subagenten einen eindeutigen
  Branch (z. B. `issue-<N>/<teilaufgabe>`) und klare Datei-/Modulverantwortung zu und
  übergibt diese Regeln im Auftrag. Rein lesende Subagenten benötigen keinen Branch.
  Bei Bedarf entsteht der Branch mit dem ersten gezielten
  `but commit -b <branch> -m "message" <change-id>...`.
- Alle Agenten bleiben im selben Workspace; kein `git checkout`, `git switch`,
  `but branch new --switch`, `git stash` oder `but teardown` während paralleler Arbeit.
  Vorhandene Worktrees nicht automatisch entfernen oder migrieren.
- Das Dateisystem, uncommittete Änderungen, Dependencies, Build-Ausgaben und laufende
  Server sind gemeinsam. Branches isolieren diese Zustände nicht. Überlappende Dateien
  und generierte Ausgaben vor dem Schreiben koordinieren; konkurrierende Änderungen
  an denselben Stellen nacheinander bearbeiten. Fremde Änderungen niemals zurücksetzen.
- Der koordinierende Agent serialisiert GitButler-Schreiboperationen: entweder führt
  er sie selbst aus, oder er weist sie jeweils genau einem Subagenten zu. Währenddessen
  pausieren Änderungen an den betroffenen Dateien. Change-IDs unmittelbar zuvor mit
  `but diff` ermitteln und nur eigene Dateien/Hunks auf den ausdrücklich genannten
  Branch committen; im gemeinsamen Workspace niemals alle Änderungen ohne IDs committen.
- Unabhängige Aufgaben bleiben auf parallelen Branches. Tatsächliche Abhängigkeiten
  ausdrücklich koordinieren und bei Bedarf mit `but move <kind> --above <basis>`
  stapeln. Die GitHub-Issue-Abhängigkeiten und der OpenSpec-Ablauf bleiben verbindlich.
- Tests im gemeinsamen Workspace prüfen die Kombination aller aktiven Branches.
  Vor einer unabhängigen Auslieferung auch die Eigenständigkeit des jeweiligen Branches
  prüfen. Workspace-weite Änderungen, History-Edits und Bereinigung erst durchführen,
  wenn alle beteiligten Agenten ihre Arbeit abgestimmt oder pausiert haben.
- Wenn Aufgaben inkompatible Checkout- oder Laufzeitzustände benötigen, die betroffenen
  Aufgaben nacheinander ausführen; nicht automatisch auf Worktrees ausweichen.

Belege: ADR-0010

Wo: `docs/ops-lab.md`, `docs/standards.md`, `docs/adr/0010-gitbutler-workspace.md`

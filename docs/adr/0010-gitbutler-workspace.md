# ADR-0010: Paralleles Arbeiten im gemeinsamen GitButler-Workspace

- **Status:** Angenommen
- **Datum:** 2026-10-04
- **Issue:** #34

## Kontext

An tollgate arbeiten mehrere Agenten gleichzeitig (Claude Code, Codex, Subagenten), jeweils
an einem eigenen Issue und OpenSpec-Change. Bisher bekam jeder Change einen eigenen
Git-Worktree. Das führte zu:

- doppelten Abhängigkeiten, Build-Ausgaben und Go-Caches pro Worktree,
- Arbeitsständen, die außerhalb des Hauptordners verwaisen,
- Konflikten, die erst beim Merge sichtbar werden statt beim Schreiben.

Außerdem lagen die Ops-lab-Werkzeuge als Kopie im Repo. Für ein später öffentliches
Repo ist das wegen ungeklärter Herkunft und Lizenz nicht tragbar.

## Entscheidung

1. Alle Agenten arbeiten im Hauptordner auf eigenen, parallel angewendeten
   GitButler-Branches (`issue-<N>/<slug>`). Keine neuen Worktrees oder Checkouts.
2. Der Koordinator vergibt disjunkte Dateiverantwortung und serialisiert
   GitButler-Mutationen. Commits enthalten nur gezielt ausgewählte eigene Change-IDs.
3. Abhängige Changes werden mit `but move <kind> --above <basis>` gestapelt, PRs mit
   `but pr new` erstellt.
4. Kombinierte Workspace-Tests ersetzen nicht die Prüfung des einzelnen Branches;
   gemergt wird nur mit grünen CI-Checks des konkreten PR-HEAD.
5. Agent-Werkzeug (Ops-lab, OpenSpec-Skills, GitButler-Skill) liegt nicht im Repo. Es wird
   lokal installiert ([`AGENT_SETUP.md`](../AGENT_SETUP.md)) und per `.gitignore`
   ausgeschlossen. Im Repo bleiben nur werkzeugneutrale Regeln: `AGENTS.md` (führend,
   `CLAUDE.md` importiert sie), [`standards.md`](../standards.md) und
   [`ops-lab.md`](../ops-lab.md).

## Alternativen

- **Ein Worktree pro Change:** echte Dateiisolation, aber doppelte Ressourcen, verwaiste
  Stände und späte Konflikte. Verworfen.
- **Ein Branch zur Zeit mit Checkout-Wechsel:** keine Parallelität für Agenten. Verworfen.
- **Ops-lab-Kopie im Repo:** reproduzierbare Werkzeuge pro Commit, aber Lizenz- und
  Herkunftsrisiko im öffentlichen Repo und doppelte Pflege der Gate-Fehler. Verworfen.

## Konsequenzen

- Branches isolieren weder Dateisystem noch laufende Server; Überschneidungen müssen vor
  dem Schreiben koordiniert werden.
- Wer das Repo klont, richtet die Agent-Werkzeuge einmal lokal ein.
- Prozesswerkzeuge können sich unabhängig von tollgate weiterentwickeln; tollgate
  legt nur fest, was der Ablauf leisten muss (`ops-lab.md`) und dass `make test` die
  Projektprüfung ist.

## Quellen

- GitButler CLI 0.22.3, `but skill install`, Konzepte „Workspace“ und „Stacks“.

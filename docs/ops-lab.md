# Agentischer Ablauf (ops-lab) in tollgate

Die Ops-lab-Werkzeuge (Skills, Rollen, `just`-Rezepte, `tools/`) liegen nicht in diesem
Repo. Sie kommen aus einem eigenen Repo und werden lokal installiert, siehe
[`AGENT_SETUP.md`](AGENT_SETUP.md); `.gitignore` schließt sie aus. Diese Datei beschreibt
den Ablauf, den tollgate davon verlangt. Die Begründung steht in
[ADR-0010](adr/0010-gitbutler-workspace.md).

## Gemeinsamer Workspace

Alle Autoren arbeiten im Hauptordner auf eigenen, parallel angewendeten GitButler-Branches.
Branches isolieren weder Dateien noch Dependencies, Server oder Build-Ausgaben. Der
Koordinator vergibt Issue, Change-ID, Branch und disjunkte Dateiverantwortung;
Überschneidungen werden nacheinander bearbeitet. Vorhandene Worktrees bleiben als
Sicherungen unangetastet und werden nicht als aktive Arbeitsräume ausgewählt.

Die Lage ergibt sich aus `but status`. Eine fehlgeschlagene Quelle bedeutet keinen freien
oder geprüften Zustand. Lokale uncommittete Dateien bleiben unzugeordnet, bis der
Koordinator sie anhand aktueller IDs zuordnet.

## Change und Gates

Ein Issue je OpenSpec-Change, ein expliziter Branch je Change (`issue-<N>/<slug>`). Der
Koordinator committet gezielt eigene Change-IDs (`but diff`, dann
`but commit -b BRANCH -m TEXT ID...`). Vorhandene Artefakte werden nicht überschrieben;
kein Checkout, Worktree, automatischer Commit oder Fetch.

Die Kette lautet: Gate 1 (Spec und Tests geplant), Tests zuerst, Gate 2 (neues Szenario
ist rot), Umsetzung, Aufgaben abhaken, Abschluss. Ein fehlender Buildbefehl ist kein
Nachweis. Gates führen die Projektprüfung von tollgate aus: `make test`. Nachweise nennen
den expliziten Branch und kennzeichnen, ob sie den gemeinsamen Workspace oder den
einzelnen Branch geprüft haben.

Der Reviewer liest Proposal, Specs, Tests und [`standards.md`](standards.md) in frischem
Kontext und berichtet Spec und Standards getrennt. Schreibende Reviewer erhalten eine
eigene Branch-/Dateizuweisung. Der Koordinator serialisiert alle GitButler-Mutationen,
während betroffene Autoren pausieren.

## Messung und Belege

Vorher/Nachher-Messungen verwenden denselben Fall und deklarierte Eingaben. Die Basis
wird per `git archive` in ein temporäres Verzeichnis exportiert; es entstehen keine
Worktrees oder separaten Checkouts. Befehle dürfen keine fremden Quellen oder
Build-Ausgaben verändern. Belege gehören zum geprüften Stand und nennen einen expliziten
Branch oder Commit, kein kombiniertes Workspace-HEAD.

## Veröffentlichung

Push, PR und Merge nur mit ausdrücklichem Nutzerauftrag. PRs entstehen mit
`but pr new <branch>`, damit Stack-Basen stimmen. Vor einem Merge:

- die tatsächlichen CI-Checks des konkreten PR-HEAD sind grün (`gh pr checks`); rot,
  ausstehend oder unbekannt blockiert,
- der Branch ist unabhängig geprüft; ein grüner Workspace-Test aller angewendeten Branches
  ersetzt das nicht,
- Änderungen an Projekt-Testbefehl (`Makefile`), CI und Gate-Regeln sind Ein-Weg-Türen
  und werden vor dem Merge von einem Menschen im Diff gelesen.

Keine Admin-Merge-Abkürzung. Nach dem Merge wird der Change archiviert. Der Koordinator
meldet die nächste konkrete Prüfung, statt selbst Branches umzusortieren, fremde Arbeit
zu committen oder Sicherungen aufzuräumen. Aktualisierung des Workspace erfolgt nach
Koordination mit `but pull`.

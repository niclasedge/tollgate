# Standards für tollgate

## Architektur und Sprache

AGENTS.md und die ADRs unter docs/adr sind maßgeblich. Prosa und OpenSpec-Artefakte sind deutsch; OpenSpec-Strukturüberschriften und SHALL/MUST bleiben englisch. Go-Bezeichner folgen Go-Konventionen.

## Projekttechnik

Go-Backend, Svelte/Vite/TypeScript-Frontend, SQLite. Keine zusätzliche Technik ohne begründete Entscheidung. Go-Tests laufen mit Race Detector über make test; generierte TypeScript-Typen werden nicht manuell gepflegt.

## Verhalten prüfen

Tests prüfen Szenarien und Invarianten, keine bloßen Dateinamen oder selbsterfüllenden Stubs. Temporäre Umgebungen entstehen im Test. Tests werden nicht gelöscht, abgeschwächt oder geskippt, um grün zu werden.

## Change-Grenzen

Ein Issue und ein expliziter GitButler-Branch je Change im gemeinsamen Workspace. proposal.md nennt alle berührten Dateien; der Koordinator vergibt disjunkte Verantwortung und serialisiert VCS-Mutationen. Kein fremder Change wird verändert. Keine Worktrees oder Checkouts anlegen; vorhandene Sicherungen bleiben erhalten. Commits enthalten nur eigene aktuelle Change-IDs. Workspace-Tests prüfen die Kombination; unabhängige Branch-Auslieferung benötigt eigene hermetische Prüfung. Changes werden nach dem Merge archiviert.

## Gates und unbekannte Zustände

Gate-Regeln haben eigene Tests. Kein Schalter umgeht ein Gate. Eine stumme Quelle und eine unbekannte Prüfung gelten nicht als bestanden. Änderungen an Gate-/Merge-Werkzeugen, Projekt-Testbefehl und CI sind Ein-Weg-Türen; vor Merge liest ein Mensch den Diff.

## Geheimnisse und Cache

Keine Tokens, Schlüssel, Wortlisten oder nicht bereinigten Payloads in Logs oder Belegen. CLIProxyAPI auth-dir enthält nur das aktive Abo. Präfixe und Konfigurationsversionen laufender Sitzungen bleiben stabil.

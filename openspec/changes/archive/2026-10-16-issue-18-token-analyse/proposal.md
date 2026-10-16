# Proposal

Issue: [#18 Token-Analyse: teuerste Bash-Befehle aus Phase-1-Daten](https://github.com/niclasedge/tollgate/issues/18)

## Why

Welche Kompressionsregeln zuerst kommen, soll nach Daten entschieden werden, nicht nach
Gefühl (ADR-0007). Die tollgate-DB speichert keine Request-Bodies, deshalb fehlt dort die
Größe der Bash-Ausgaben.

## What Changes

- `docs/token-analyse.md`: Rangliste der Bash-Befehlsgruppen nach Ausgabe-Bytes aus lokalen
  Claude-Code-Transkripten (nur gelesen), mit Methode, Anteil über `min_bytes`, rtk-Anteil,
  Pipelines und Ketten sowie einer Empfehlung für #19 und #20.
- Ergebnis: Dateilesen per Bash (`sed -n`, `cat`, `head`, `tail`, 39,5 %) bleibt unberührt,
  `search` (`grep`, `rg`, `git grep`, 17,3 %) ist gerechtfertigt, `diff` folgt, Test- und
  Build-Regeln bleiben mit kleinerem Anteil, `min_bytes = 2048` bleibt.

## Capabilities

### New Capabilities

- `compression`: Rangliste als Grundlage der Regelauswahl.

## Impact

- Nur Doku: `docs/token-analyse.md`. Keine Projektnamen, Pfade oder Inhalte aus den
  Transkripten, nur Werkzeugnamen und Summen.

## Was ausdrücklich nicht

- Kein Speichern von Bodies in der DB, um die Analyse zu wiederholen.

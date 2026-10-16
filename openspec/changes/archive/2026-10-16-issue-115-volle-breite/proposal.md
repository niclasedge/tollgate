## Why
Der Inhaltsbereich ist auf `max-width: 64rem` begrenzt. Auf breiten Bildschirmen bleibt rechts viel Fläche leer, und in der Übersicht rutscht die dritte Abo-Kachel in eine neue Zeile, obwohl drei nebeneinander passen. Issue: https://github.com/niclasedge/tollgate/issues/115.
## What Changes
- Die Shell begrenzt den Inhaltsbereich nicht mehr auf eine feste schmale Breite; alle Bereiche nutzen die verfügbare Breite neben der Sidebar.
- Die Abo-Kacheln der Übersicht laufen als `auto-fit`/`minmax`-Grid mit einer Mindestbreite, bei der drei Kacheln ab 1440 px in eine Zeile passen; auf dem iPhone stehen sie untereinander.
- Fließtext-Absätze bleiben für die Lesbarkeit auf eine Zeilenlänge von etwa 75 Zeichen begrenzt; Tabellen, Listen und Grids dürfen breit werden.
## Capabilities
### Modified Capabilities
- `frontend-views`: Layout-Breite und Kachel-Grid.
## Impact
Branch `issue-115/volle-breite`, gemeinsamer Workspace. Dateien: `web/src/lib/components/Shell.svelte`, `web/src/lib/views/Home.svelte` (nur Grid-Styles), `web/src/layout.test.ts`, diese Artefakte. Keine API- oder Backend-Änderung.

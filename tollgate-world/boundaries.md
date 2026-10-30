# Grenzen dieser World

## Was diese World abdeckt

Wo Code, Tests, Doku und Werkzeuge von tollgate liegen, welche Regeln für Pipeline, Redactor, Frontend und Zusammenarbeit gelten und woher der Auftrag kommt. Jede Aussage hat eine Adresse in der Coverage-Map von `world.md`.

## Was diese World nicht abdeckt

- **Architekturentscheidungen:** stehen in den ADRs unter `docs/adr/` und in `docs/ARCHITECTURE.md`. Neue Entscheidungen brauchen ein neues ADR; der Mensch gibt frei.
- **Verhalten im Detail:** die Anforderungen stehen in `openspec/specs/`, laufende Änderungen in `openspec/changes/`.
- **Mission:** die aktuelle Arbeit und ihre Reihenfolge. Quelle: offene GitHub-Issues, Milestones und `openspec list`; immer live abfragen.
- **Betrieb und Deploy:** `docs/DEPLOY.md` und `just --list`. Deploy und Veröffentlichung gibt der Mensch frei.
- **Lore:** was ein Agent durch Arbeit gelernt hat. Liegt in seiner Memory, nicht hier.
- **Zahlen und Zustand:** Anzahl Tests, offene Issues, Token-Messwerte, Laufzeiten. Immer mit den Befehlen der Coverage-Map ableiten, nie aus diesen Dateien.

## Wenn keine Quelle antwortet

Die Lücke melden und den Menschen fragen. Keine plausible Antwort erfinden.

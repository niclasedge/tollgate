## Kontext
`main` in `Shell.svelte` hat `max-width: 64rem`. Bei 1440 px bleiben davon nach Padding rund 56rem; die Abo-Kacheln (`minmax(min(100%, 24rem), 1fr)`) passen nur zu zweit hinein.
## Entscheidung
- `max-width` an `main` entfällt. Das Padding (`clamp(1.25rem, 5vw, 4rem)`) bleibt und hält den ruhigen Rand.
- Abo-Kacheln: Mindestbreite `20rem` statt `24rem`. Bei 1440 px und ausgeklappter Sidebar bleiben rund 68rem, drei Kacheln brauchen 61,8rem. `min(100%, …)` bleibt, damit auf dem iPhone keine Kachel überläuft.
- Lesbarkeit: In `Shell.svelte` gilt `main :global(p) { max-width: 75ch; }` (Layout-Regel bei der Shell; `app.css` liefert Vitest per `?raw` leer, daher wäre sie dort nicht testbar). Absätze (Hinweise, Fehlermeldungen) bleiben kurz; Tabellen, Grids und Listen werden nicht begrenzt.
- Bereits vorhandene `auto-fit`-Grids (Heute-Kacheln, Kosten-Summen, Limits) bleiben unverändert und profitieren von der Breite.
## Alternativen
- Hohe feste Grenze (z. B. 120rem): verschiebt das Problem nur auf sehr breite Monitore; das Issue verlangt die volle Breite.
- Kachelzahl per Media Query festlegen: starrer als `auto-fit` und bei eingeklappter Sidebar falsch.
## Risiken
Keine Logik-Änderung. Sehr breite Zeilen in Listen bleiben über die festen Grid-Spalten lesbar; Screenshots belegen 1440 px, 1920 px und 390 px in hell und dunkel.

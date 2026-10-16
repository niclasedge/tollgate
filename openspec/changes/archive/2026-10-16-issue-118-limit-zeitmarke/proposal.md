# Proposal

Issue: [#118 Abo-Limits: Zeitmarke im 5-h-Fenster, aktives Abo grün umrandet](https://github.com/niclasedge/tollgate/issues/118)

## Why

Beim Wochenlimit zeigt Accounts mit einem Strich, wie viel der Woche schon vergangen ist. So
lässt sich der Verbrauch mit der Zeit vergleichen. Für das 5-Stunden-Fenster fehlt diese
Marke, ebenso in der Übersicht für beide Fenster. Außerdem hebt sich das aktive Abo in der
Übersicht mit dem blassen Akzent kaum ab.

## What Changes

- **Zeitmarke 5 Stunden:** `LimitCard` (Übersicht) und `AccountUsage` (Accounts und
  Abo-Karten der Übersicht) zeigen im 5-Stunden-Balken einen Strich für „jetzt“. Vergangener
  Anteil = (jetzt − (Reset − 5 h)) / 5 h. Darunter steht „Strich = jetzt · X % des Fensters
  vergangen“.
- **Zeitmarke Woche in der Übersicht:** `LimitCard` bekommt die Marke auch für die Woche,
  damit beide Fenster gleich aussehen.
- **Keine Marke** ohne gültigen Reset, bei abgelaufenem Reset, bei einem Reset weiter als die
  Fensterlänge entfernt oder ohne Messwert.
- **Gemeinsame Hilfsfunktion** `elapsedPercent` in `web/src/lib/limits.ts` für den Anteil.
- **Aktives Abo grün umrandet:** Die aktive Karte bekommt eine Outline in der Statusfarbe
  „ok“ (`--c-ok`) statt `--c-accent-soft`, in `LimitCard` und in `AccountCard` (die Übersicht
  zeigt mit Account-Verwaltung `AccountCard`).

## Capabilities

### Modified Capabilities

- `frontend-views`: Zeitmarke in beiden Limit-Fenstern, grüne Outline der aktiven Abo-Karte.

## Impact

Nur Frontend, keine API- oder Schemaänderung. Berührte Dateien:

- Neu: `web/src/lib/limits.ts`, `web/src/limit-marks.test.ts`, diese Change-Artefakte.
- Geändert: `web/src/lib/components/LimitCard.svelte`,
  `web/src/lib/components/AccountUsage.svelte`,
  `web/src/lib/components/AccountCard.svelte` (eine CSS-Zeile für die Outline).

## Was ausdrücklich nicht

- Keine Prognose („reicht bis …“), nur der Vergleich Verbrauch zu vergangener Zeit.
- Keine neue Farbe: Strich und Outline nutzen vorhandene Tokens (`--c-text`, `--c-ok`).

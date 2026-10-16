# Proposal

Issue: [#123 Tageswerte über Kosten- und Tokens/s-Diagramm, beide Diagramme auf der Übersicht](https://github.com/niclasedge/tollgate/issues/123)

> **Gestapelt auf #121** (Kosten & Performance). Dessen Requirements sind noch nicht archiviert;
> dieser Change ergänzt sie deshalb als neue Requirements.

## Why

Die Kostenkurve und das Tokens/s-Diagramm zeigen Stunden. Wie viel ein ganzer Tag gekostet hat
oder wie schnell ein Modell an einem Tag war, muss man aus der Aufschlüsselung zusammensuchen.
Auf der Übersicht fehlen beide Kurven ganz.

## What Changes

- Über der Kostenkurve steht je UTC-Tag die Kostensumme (aus den Tageszeilen, mit „≥“ bei
  Modellen ohne Preis).
- Über dem Tokens/s-Diagramm steht je UTC-Tag der Tagesmedian je Modell (aus
  `days[].tps_median_milli`), eine Zeile je Modell in der Farbe seiner Linie.
- Gestrichelte Trennlinien markieren Mitternacht. Beschriftungen, die sich überdecken würden,
  entfallen, der jüngste Tag zuerst.
- Die Übersicht zeigt unter den Abo-Limits den Bereich „Verlauf“ mit beiden Diagrammen der
  letzten 7 Tage. Er lädt mit der Seite neu, ein Fehler betrifft nur sein Diagramm.

## Capabilities

### Modified Capabilities

- `frontend-views`: Tageswerte über den Stundendiagrammen, Verlauf auf der Übersicht.

## Impact

- Frontend: `web/src/lib/hours.ts` (`daySpans`, `dayBands`), `costs.ts` (`dayCosts`), `perf.ts`
  (`TpsLine.days`), `components/CostChart.svelte`, `components/TpsChart.svelte`, neu
  `components/TrendCharts.svelte`, `views/Costs.svelte`, `views/Home.svelte`, Tests.
- Keine Backend-Änderung: Tagessummen und Tagesmediane liefern `/api/costs` und
  `/api/performance` bereits.

## Was ausdrücklich nicht

- Kein Umrechnen auf die lokale Zeitzone: Tage bleiben UTC.
- Kein Mittel über Modelle hinweg: Tokens/s unterscheiden sich je Modell zu stark.

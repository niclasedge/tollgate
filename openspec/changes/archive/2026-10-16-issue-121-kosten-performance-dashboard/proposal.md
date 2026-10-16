# Proposal

Issue: [#121 Kosten & Performance zusammenlegen: Dashboard mit Graphen je Account, Details darunter](https://github.com/niclasedge/tollgate/issues/121)

> **Gestapelt auf #87/#116** (Performance-Ansicht, Tokens/s-Diagramm). Die offenen Changes
> `issue-87-performance` und `issue-116-tps-trend` fügen die Performance-Ansicht erst hinzu;
> dieser Change formuliert die zusammengelegte Seite deshalb als neue Requirements statt als
> Änderung nicht archivierter Requirements.

## Why

Kosten und Performance liegen heute in zwei Bereichen. Wer wissen will, was ein Abo gerade
kostet und wie schnell es antwortet, muss zwischen beiden springen. Die Kostenansicht schlüsselt
zuerst nach Modell auf, zeigt leere Tage als leere Balken und hat keine Kurve über den Tag.
Gewünscht ist eine Seite: oben ein Dashboard mit den wichtigsten Zahlen und Graphen je
**Account**, darunter die Details bei Bedarf.

## What Changes

- **Navigation:** Ein Bereich „Kosten & Performance“ unter `#/costs` (Taste `3`); `#/performance`
  leitet dorthin um, der eigene Eintrag und die Taste `8` entfallen. Zeitraum Heute / 7 Tage /
  30 Tage, Standard 7 Tage.
- **Dashboard oben:** Gesamtkosten groß mit Anzahl Anfragen und Aufschlüsselung je Account
  (Kosten, Anteil, Tokens); Kostenkurve als gestapeltes Flächendiagramm je Account (stündlich bei
  Heute/7 Tage, täglich bei 30 Tagen); Kennzahlenzeile (Tokens verarbeitet, Cache gelesen,
  Eingabe ungecacht, Ausgabe, Cache-Ersparnis, von tollgate gespart); Tokens/s-Diagramm aus #116
  mit TTFT-Median und Anteil „langsam“ je Modell.
- **Details unten:** Aufschlüsselung mit Umschalter Account / Modell / Client / Tag (Tage ohne
  Wert ausgeblendet), darunter die Performance-Details aus #87/#116 unverändert (Modelltabelle mit
  Aufklappen, Redactor-Kosten, Speedtest).
- **Backend:** `GET /api/costs` liefert zusätzlich Stunden-Buckets (Stunde × Account × Modell) für
  Zeiträume bis 7 Tage und `cache_savings_nano_usd` je Zeile. `meter` berechnet die
  Cache-Ersparnis, der Store aggregiert je UTC-Stunde.
- Accounts tragen im Dashboard denselben Namen wie im Bereich Accounts.

## Capabilities

### Modified Capabilities

- `dashboard-api`: Kostenbericht mit Stunden-Buckets und Cache-Ersparnis.
- `meter`: Cache-Ersparnis je Usage.
- `store`: Aggregation je UTC-Stunde, Account und Modell.
- `frontend-shell`: Navigation mit zusammengelegtem Bereich und Umleitung.
- `frontend-views`: Bereich „Kosten & Performance“ mit Dashboard und Aufschlüsselung.

## Impact

Gemeinsamer Workspace, Branch vom Koordinator. Berührte Dateien:

- Backend: `internal/meter/cost.go` (+ Test), `internal/store/traces.go` (`CostByHour`, + Test),
  `internal/dashboard/{handler,types}.go` (+ Test), generiert `web/src/lib/types/dashboard.ts`.
- Frontend: `web/src/lib/views/Costs.svelte` (Dashboard + Details), `web/src/lib/views/Performance.svelte`
  (nur noch Detailbereich ohne eigenen Abruf), neu `web/src/lib/components/CostChart.svelte`,
  `web/src/lib/costs.ts`, `web/src/lib/router.svelte.ts`, `web/src/App.svelte`, Tests.
- Doku: `docs/ARCHITECTURE.md` (Kostenbericht), diese Change-Artefakte.

## Was ausdrücklich nicht

- Keine neue Tabelle, keine Migration: die Stunden kommen aus `traces`.
- Keine Farbe außer dem Akzentblau; Accounts unterscheiden sich über Grautöne und Transparenz.
- Keine Änderung an Live, Übersicht oder den Performance-Berechnungen aus #87/#116.
- Kein Umrechnen auf die lokale Zeitzone: Tage und Stunden bleiben UTC und sind so beschriftet.

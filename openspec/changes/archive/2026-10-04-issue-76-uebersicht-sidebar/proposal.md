# Proposal

Issue: [#76 Frontend: Übersichts-Startseite und gruppierte Sidebar](https://github.com/niclasedge/tollgate/issues/76)

Geht zurück auf GitHub-Issue #76. Abhängigkeit #10 ist geschlossen.

## Why

Wer das Frontend öffnet, landet in Live und muss sich Abo-Limits, Tageswerte und Health aus drei Bereichen zusammensuchen. Die Sidebar zeigt sieben gleichrangige Icons ohne Beschriftung und ohne Ordnung.

## What Changes

- Neue Standardroute `#/home` „Übersicht“ mit Taste `0`; die Tasten `1`–`7` bleiben unverändert.
- Startseite mit Abo-Limit-Karte (5 Stunden und Woche: Prozent groß, Balken, „zurückgesetzt in …“ mit Datum, leerer Zustand), Heute-Kennzahlen aus `/api/costs?days=1`, Health (tollgate, CLIProxyAPI, aktiver Account) und den letzten 10 Anfragen mit Link ins aufgeklappte Detail in Live (`#/live/<id>`).
- Neue Komponente `LimitCard` für genau ein Abo, damit #29 sie je Abo wiederverwenden kann.
- Sidebar in Gruppen (Übersicht, Beobachten, Steuern, Verwalten) mit Icon und Label, dezenten Zählern für Clients und Accounts und einem Einklapp-Knopf; der Zustand liegt in `localStorage`. Mobil bleibt die untere Leiste.
- Live öffnet das Detail der Anfrage aus `#/live/<id>`.

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `frontend-shell`: Navigation mit Startseite, Gruppen, Zählern und Einklappen.
- `frontend-views`: Startseite „Übersicht“, Detail-Link in Live.

## Was ausdrücklich nicht

- Keine Backend-Änderung; fehlende Werte erscheinen als „unbekannt“.
- „Letzte 10 Redactions“ erst mit #16, jetzt wird dafür nichts angezeigt.
- Mehrere Abos-Karten erst mit #29.

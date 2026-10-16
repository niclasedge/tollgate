# Design

## Router und Tasten

`routes` bekommt den Eintrag `home` („Übersicht“, Taste `0`) an erster Stelle und je Eintrag eine Gruppe. `parseHash` fällt bei leerem oder unbekanntem Hash auf `home` zurück. Die Tasten `1`–`7` bleiben, damit gewohnte Kürzel nicht wandern; `0` liegt auf der Tastatur ohnehin vor `1`. `hrefFor(id, detail?)` baut `#/live/<id>`.

| Gruppe | Bereiche |
|---|---|
| Übersicht | Übersicht |
| Beobachten | Live, Anfragen, Kosten |
| Steuern | Filter, Kompression |
| Verwalten | Clients, Accounts |

## Sidebar

- Ausgeklappt: Gruppenüberschrift klein in Großbuchstaben, darunter Icon und Label; der aktive Eintrag ist mit dem weichen Akzentton hinterlegt. Zähler stehen grau rechts (Clients: `/api/clients`, Accounts: `status.accounts`), nur mit Admin-Token, Fehler lassen den Zähler weg.
- Eingeklappt: nur Icons wie bisher, Label als `title`. Der Knopf „Seitenleiste einklappen/ausklappen“ trägt `aria-expanded`, der Zustand liegt unter `tollgate.sidebar` in `localStorage`.
- Unter 720 px bleibt die untere Leiste mit Icons; Gruppenüberschriften, Zähler und Einklapp-Knopf entfallen dort.

## Startseite

Lädt `api.status()`, `api.costs(1)` und `api.traces({ limit: 10 })` parallel und alle 30 s neu. Jeder Abschnitt zeigt seinen eigenen Fehler, ein Fehler blendet die anderen nicht aus.

- **Abo-Limits:** `LimitCard` mit `title`, `limits: RateLimits | null` und optional `active`. Je Fenster: Prozent groß, Balken (`role="meter"`, Farbe nach Status: Akzent, gelb, rot), „zurückgesetzt in 2 h 13 min · 04.10., 18:30“. Ohne Messung: „Noch keine Daten“. #29 rendert eine Karte je Abo und setzt `active`.
- **Heute (UTC):** Anfragen, Tokens ein (`input + cache_read + cache_creation`), Tokens aus, Cache-Trefferquote, Gespart, Kosten.
- **Health:** tollgate, CLIProxyAPI mit runden Statuspunkten; aktiver Account ist der einzige Eintrag in `status.accounts`. Kein Eintrag: „kein Account“; mehrere: „unklar“ mit gelbem Punkt.
- **Letzte Anfragen:** kompakte Zeilen (Status, Zeit, Client, Modell, Tokens) als Links auf `#/live/<id>`; Live klappt diese Anfrage auf.
- **Letzte Redactions:** nicht gerendert, bis #16 die Daten liefert.

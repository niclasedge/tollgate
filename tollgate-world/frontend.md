# Frontend · Oberflaeche

## Auf einen Blick

Das Frontend ist eine Svelte-App (Vite, TypeScript), statisch gebaut und per `embed` im tollgate-Binary. Es zeigt Live-Anfragen, Kosten, Filter, Kompression, Clients und Accounts, nur aus dem Tailnet und mit Admin-Token. Hier: Technik, Funktionen, Design und die Client-Einrichtung.

## Technik

- Svelte mit Vite und TypeScript, statischer Build.
- TypeScript-Typen werden aus den Go-Structs generiert (z. B. mit `tygo`), nie von
  Hand gepflegt.
- Der Build wird per `embed` ins tollgate-Binary eingebettet, ein Deployment-Artefakt.
- Live-Daten per SSE, YAML-Editor mit CodeMirror und Schema-Validierung.
- Responsiv, damit die Oberfläche auch auf dem iPhone im Tailnet nutzbar ist.

Belege: ADR-0003

Wo: `web/`, `web/embed.go`, `tygo.yaml`, `Makefile` (Targets web und generate)

## Funktionen

- **Live:** Liste der Anfragen mit Status, Modell, Tokens und Ersparnis.
- **Anfragen:** Detailansicht mit Stufen-Timeline, Diff pro Stufe und Rückweg.
- **Kosten:** Tokens pro Tag, Client und Modell, Cache-Trefferquote, gesparte Tokens.
- **Filter:** Redactor-Konfiguration mit Liste, Editor, Wortliste (maskiert),
  Allowlist, Live-Test (Text einfügen, Treffer sehen), Versionshistorie mit Rollback
  sowie YAML-Import und -Export.
- **Kompression:** Regeln pro Befehl mit Playground, der das Ergebnis jeder Stufe zeigt.
- **Clients:** Keys erzeugen, sperren und rotieren, mit Setup-Snippet pro Client.
- **Accounts:** aktiver Account, Health von tollgate und CLIProxyAPI.
- Zugriff nur aus dem Tailnet, zusätzlich mit Admin-Token.

Wo: `web/src/lib/views/`, `web/src/lib/components/`, `openspec/specs/frontend-views/spec.md`, `internal/dashboard/`

## Design

Design inspiriert von Things 3, ohne dessen Logos, Icons, Namen oder Assets zu
übernehmen:
- Hell und ruhig, viel Weißraum, große klare Titel, dezente Typografie.
- Schmale Sidebar links mit Icons für die Bereiche oben.
- Ein einziger dezenter Blauton als Akzent, sonst Grautöne. Keine schweren Rahmen.
- Runde Statusindikatoren statt greller Badges, sanfte und kurze Übergänge.
- Detailansichten klappen inline auf, statt neue Seiten zu öffnen.
- Vollständige Tastatursteuerung und ein sauberer Dark Mode.

Wo: `web/src/app.css`, `web/src/lib/theme.svelte.ts`

## Client-Einrichtung

- Ein Client braucht nur zwei Umgebungsvariablen: `ANTHROPIC_BASE_URL` (tollgate im
  Tailnet) und `ANTHROPIC_AUTH_TOKEN` (sein Client-Key von tollgate).
- Prüfe in der aktuellen Claude-Code-Dokumentation, welche Variablen genau gelten.
- Das Frontend erzeugt pro Client einen Key und zeigt ein kopierfertiges Setup-Snippet.
- Keys lassen sich im Frontend sperren und rotieren.

Anleitung: `docs/DEPLOY.md`, Abschnitt „Client anlegen“; Key per `just client <name>`.

Belege: ADR-0009

Wo: `docs/DEPLOY.md`, `internal/auth/`, `deploy/setup.sh`

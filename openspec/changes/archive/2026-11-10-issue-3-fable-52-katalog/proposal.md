# Proposal

Issue: [#3 CPA: claude-fable-5-2 im Katalog-Overlay ergänzen](https://github.com/niclasedge/tollgate/issues/3)

## Why

Claude Code bekommt über tollgate `400 unknown provider for model claude-fable-5-2`. Der eingebettete CPA-Katalog am Pin `8ef43e4` kennt Fable 5.2 nicht; erst CPA v8.0.63 bettet es ein. Ein Pin-Update bräuchte die volle Prüfung aus `UPSTREAM.md`; ADR-0020 sieht für fehlende Modelle einen weiteren Overlay-Eintrag vor.

## What Changes

- Das Overlay wird eine Tabelle. Neuer Eintrag `claude-fable-5-2`, geklont aus `claude-fable-5-1`, Thinking nur adaptiv wie in CPA v8.0.63.
- Build-Test `TestTollgateOverlay` ersetzt `TestTollgateHaiku55` und prüft alle Einträge.
- Meter: Preis für `claude-fable-5-2` wie Fable 5.1.
- Nachtrag in ADR-0020, `PATCHES.md`, `NOTICE.md`, `UPSTREAM.md`.

## Impact

- Capability `cpa-model-overlay` (neues Szenario), `meter` unverändert in der Anforderung.
- Routing, Cloaking und Egress bleiben unverändert.

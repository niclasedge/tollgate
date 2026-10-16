# ADR-0020: Katalog-Overlay für Modelle, die der CPA-Pin nicht kennt

- **Status:** Angenommen
- **Datum:** 2026-10-09
- **Issue:** #137
- **Ergänzt:** [ADR-0016](0016-cpa-refresh-barriere.md) (gleicher Bridge-Mechanismus),
  [ADR-0017](0017-cpa-egress-nur-anthropic.md) (Egress bleibt unverändert)

## Kontext

CPA nimmt nur Modelle an, die in seiner Model-Registry stehen. Unbekannte IDs beantwortet
er lokal mit 400, ohne Anthropic zu fragen. Die Registry kommt aus dem eingebetteten
`internal/registry/models/models.json`, zur Laufzeit ergänzt durch einen entfernten Katalog
(`router-for-me/models`). Den entfernten Katalog schalten wir mit `-local-model` ab, und
ADR-0017 lässt ihn auch nicht durch die Egress-Regeln.

`claude-haiku-5-5` steht nur im entfernten Katalog, nicht im eingebetteten Katalog am Pin
`8ef43e4` und auch in keiner neueren CPA-Version. Ein Speedtest gegen das Modell ergibt 400
in 0–1 ms. `model-alias` hilft nicht: Ein Alias zeigt auf ein schon bekanntes Modell, und
CPA ruft upstream dessen Namen auf, also Haiku 4.5.

Optionen:

1. Entfernten Katalog erlauben: Das öffnet Egress zu GitHub, und ein fremder Katalog
   ändert die Registry zur Laufzeit. Das widerspricht ADR-0017.
2. Auf ein CPA-Release warten: offen, wie lange.
3. Eigener Eintrag im temporären Build (dieses ADR).

## Entscheidung

Option 3, nach dem Muster von ADR-0016:

- `deploy/cliproxyapi/bridge/models_overlay.go.txt` wird nur im temporären Build
  nach `internal/registry/tollgate_models_overlay.go` kopiert. Ein `init()` läuft nach
  dem Laden des eingebetteten Katalogs, weil Go die Dateien eines Pakets in
  Namensreihenfolge initialisiert.
- Das Overlay klont den eingebetteten Eintrag `claude-sonnet-5-5` und setzt nur
  ID und Anzeigenamen. Die Werte passen zu Haiku 5.5: gleiche Generation, 1M Kontext,
  128K Ausgabe, adaptive thinking mit den Stufen `low` bis `max`, Websuche.
- Kennt der Katalog `claude-haiku-5-5` schon, macht das Overlay nichts. Fehlt die Basis,
  ebenfalls nichts; dann schlägt der Build-Test fehl.
- `TestTollgateHaiku55` läuft im Image-Build mit Race Detector neben dem Test aus
  ADR-0016.
- Aus `router-for-me/models` wird nichts kopiert, weil das Repo keine Lizenz hat. Der
  Eintrag wird aus dem MIT-lizenzierten eingebetteten Katalog abgeleitet.

## Folgen

- Claude Code kann `claude-haiku-5-5` über tollgate nutzen.
- Routing, Cloaking, Token-Erneuerung und Payloads bleiben unverändert. Das Overlay
  ändert nur die Liste bekannter Modelle.
- Bettet ein Pin-Update das Modell ein, wird das Overlay wirkungslos. Dann werden
  Overlay, Test, `apply.sh`-Zeilen und Dockerfile-Test entfernt, und dieses ADR wird abgelöst.
- Weitere fehlende Modelle kommen mit einem eigenen Issue und Eintrag in dieselbe Datei.

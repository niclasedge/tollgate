# Proposal

Issue: [#137 CPA: claude-haiku-5-5 im eingebetteten Katalog ergänzen (Build-Overlay)](https://github.com/niclasedge/tollgate/issues/137)

## Why

CPA lehnt `claude-haiku-5-5` lokal mit 400 ab, weil der eingebettete Katalog am Pin das Modell nicht kennt. Den entfernten Katalog schließen `-local-model` und ADR-0017 aus. Claude Code kann Haiku 5.5 deshalb über tollgate nicht nutzen.

## What Changes

- Neues Build-Overlay `deploy/cliproxyapi/bridge/models_overlay.go.txt`. Es klont im temporären Build den eingebetteten Eintrag `claude-sonnet-5-5` als `claude-haiku-5-5`.
- Kennt der Katalog das Modell schon, bleibt das Overlay wirkungslos.
- `apply.sh` kopiert Overlay und Test. Der Image-Build führt `TestTollgateHaiku55` mit Race Detector aus.
- ADR-0020 und `third_party/cpa-model-overlay/PATCHES.md` halten Entscheidung und Herkunft fest. `UPSTREAM.md` und `NOTICE.md` verweisen darauf.

## Impact

- Neue Capability `cpa-model-overlay`.
- Kein Go-Code in tollgate. Routing, Cloaking und Egress bleiben unverändert.

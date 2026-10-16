## ADDED Requirements

### Requirement: Fehlende Modelle im CPA-Katalog
Der CPA-Image-Build SHALL Modelle, die der eingebettete Katalog am Pin nicht kennt, nur in der temporären Quellkopie als Katalogeintrag ergänzen, abgeleitet aus einem eingebetteten Eintrag derselben Generation.

#### Scenario: Haiku 5.5 fehlt am Pin
- **WHEN** der eingebettete Katalog `claude-haiku-5-5` nicht enthält
- **THEN** listet CPA `claude-haiku-5-5` genau einmal als Claude-Modell mit 1M Kontext, 128K Ausgabe und adaptive thinking

#### Scenario: Upstream bettet das Modell ein
- **WHEN** der eingebettete Katalog `claude-haiku-5-5` bereits enthält
- **THEN** ändert das Overlay den Katalog nicht

#### Scenario: Basis fehlt
- **WHEN** der Basiseintrag `claude-sonnet-5-5` am Pin fehlt
- **THEN** schlägt der Overlay-Test fehl und der Image-Build bricht ab

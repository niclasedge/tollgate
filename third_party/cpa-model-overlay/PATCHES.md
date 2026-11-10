# CPA-Katalog-Overlay (#137)

Hier liegt kein Upstream-Code. Basis und MIT-Copyright stehen in `../NOTICE.md` und
`../../deploy/cliproxyapi/UPSTREAM.md`; die Lizenz wurde vor der Änderung geprüft.
ADR: `../../docs/adr/0020-cpa-katalog-overlay.md`.

`apply.sh` kopiert `deploy/cliproxyapi/bridge/models_overlay.go.txt` und den
zugehörigen Test nur in die temporäre Go-Modul-Kopie im Build. Das Overlay ergänzt
`claude-haiku-5-5` als Klon des eingebetteten `claude-sonnet-5-5` mit eigener ID und eigenem
Anzeigenamen, und `claude-fable-5-2` als Klon von `claude-fable-5-1` mit nur adaptivem
Thinking wie im Katalog von CPA v8.0.63 (#3). Grund: Der eingebettete Katalog am Pin kennt das Modell nicht, und den
entfernten Katalog schließen `-local-model` und ADR-0017 aus.

Es gibt keine Änderung an Upstream-Dateien, Routing, Cloaking oder Requests. Aus dem
lizenzlosen `router-for-me/models` wird nichts übernommen. Der Image-Build prüft
`TestTollgateOverlay` mit Race Detector. Bei jedem Pin-Update prüfen, ob die Modelle
inzwischen eingebettet sind; dann deren Einträge (zuletzt das Overlay) entfernen.

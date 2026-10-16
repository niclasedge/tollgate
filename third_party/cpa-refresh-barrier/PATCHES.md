# CPA Refresh-Barriere (#77)

Kein Upstream-Code wird hier gespeichert. Basis und MIT-Copyright stehen in
`../NOTICE.md` und `../../deploy/cliproxyapi/UPSTREAM.md`; Lizenz vor Änderung geprüft.
ADR: `../../docs/adr/0016-cpa-refresh-barriere.md`.

Die selbst geschriebenen Dateien unter `deploy/cliproxyapi/bridge` werden nur in eine
temporäre Go-Modul-Kopie im Build übernommen. `apply.sh` fügt eine Sperrprüfung nach
dem vorhandenen Refresh-Mutex und eine Management-Route ein. Beide Einfügestellen
werden auf genau einen Treffer geprüft. Grund: Single-Use-Refresh-Tokens dürfen während
Download/Delete nicht rotieren; die vorhandene Disabled-API schützt nicht.

Keine Änderung an Routing, Cloaking, Token-Erneuerungsprotokoll oder normalen Requests.
Regressionstest wartet eine laufende Rotation ab und verhindert danach einen erneuten
Executor-Aufruf, bis Resume erfolgt. Der Image-Build prüft den Test mit Race Detector
und baut das eigentliche Binary weiterhin ohne cgo.

Die Pause kann die bekannte Datei-ID vor Registrierung reservieren. Dies schützt auch
den Zielaccount bis zur Verifikation. Persistierungsfehler bestätigen keine Pause.
Neustart und mehrdeutige Resume-Antworten werden im Gateway zusätzlich geprüft.

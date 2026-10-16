# Tasks

Zusammen mit #19.

## 1. Strategien

- [x] 1.1 Tests zuerst je Paket (tabellengetrieben, Golden, Determinismus); rot sehen.
- [x] 1.2 `logs`: Level-Klassifikator, Stacktrace-Automat, Summary-Erkennung, Auswahl, Dedupe, Vorlagen.
- [x] 1.3 `diff`: Parser, Lockfile- und Whitespace-Hunks, Grenzen für Dateien und Hunks, Kontext.
- [x] 1.4 `search`: Formen erkennen, Grenzen pro Datei und für Dateien, lange Zeilen kürzen; Treffer ohne Pfad bleiben unverändert.
- [x] 1.5 Registrierung in `compress.go`, Regel `search` ohne `find`.

## 2. Herkunft und Doku

- [x] 2.1 `third_party/headroom/LICENSE` und `NOTICE`, Eintrag in `third_party/NOTICE.md`, Dateiköpfe mit Commit `0947a28`.
- [x] 2.2 Abschnitt Kompression in `docs/ARCHITECTURE.md`.

## 3. Abschluss

- [x] 3.1 `make test`, `make lint` grün.

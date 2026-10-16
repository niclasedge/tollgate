# Tasks

## 1. Backend

- [x] 1.1 Tests zuerst: `Explain` entscheidet wie `Compress` für jedes Gate, zeigt die Schritte einer echten Strategie, `RuleList` markiert eingebaute Regeln.
- [x] 1.2 `explain.go`, `types.go`, `Rule.Builtin` in `WithDefaults`.
- [x] 1.3 API-Tests: Regeln der aktiven Version, Playground mit aktiver und festgehaltener Version, 404, unbekannte Felder, kein Text im Log.
- [x] 1.4 Routen in `redactcfg`, `CompressEnabled` aus der Konfiguration, tygo-Eintrag.

## 2. Frontend

- [x] 2.1 Tests zuerst (`compression.test.ts`): Regelliste, Status aus, Schritte inline, Grund bei unveränderter Ausgabe.
- [x] 2.2 `Compression.svelte`, Route in `App.svelte`, API-Methoden.

## 3. Abschluss

- [x] 3.1 `make test`, `make lint`, `pnpm check` grün; Seite im Browser angesehen.

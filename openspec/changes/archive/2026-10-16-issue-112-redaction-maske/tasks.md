## 1. Tests zuerst (Gate 2)
- [x] 1.1 Tabellentest `TestMask` in `internal/redact/mask_test.go`: je Typ (E-Mail, API-Key, JWT, IPv4/IPv6, Connection-Passwort, Private Key, Wortliste, eigene Regex, Heuristik) erwartete Maske, Invarianten (Segment, Viertel, 64 Bytes, ohne Klarzeichen) und Determinismus.
- [x] 1.2 Stufentest: Treffer der Redact-Stufe tragen Masken, Body unverändert gegenüber dem Stand ohne Maske.
- [x] 1.3 Store-Test: Maske ohne `•` abgelehnt, Maske in `RecentRedactions`.
- [x] 1.4 Dashboard-Test: `mask` in Trace-Detail und `/api/redactions`.
- [x] 1.5 `TestNoPlaintextInDBLogsAndAdminAPI`: jede Redaction und jeder Treffer hat eine Maske, keine enthält einen Kanarienwert.
- [x] 1.6 Frontend-Tests in `web/src/timeline.test.ts` und `web/src/home.test.ts`.
- [x] 1.7 Rot-Ausgabe in `gate2.md`.

## 2. Umsetzung
- [x] 2.1 `redact.Mask` und Aufruf in `Number`.
- [x] 2.2 Feld `Mask` in `pipeline.Match`, `store.Match`, Validierung, `RecentRedactions`, Server-Mapping.
- [x] 2.3 Feld `mask` in `dashboard.Match`/`Redaction`, `make generate`.
- [x] 2.4 Timeline und Übersicht zeigen die Maske (hell/dunkel, responsiv).

## 3. Abschluss
- [x] 3.1 `make lint`, `make test` (mit Paritätstest), `make build`, `openspec validate issue-112-redaction-maske --strict` grün.
- [x] 3.2 Screenshots desktop/iphone × hell/dunkel unter `.claude/belege/issue112/`.

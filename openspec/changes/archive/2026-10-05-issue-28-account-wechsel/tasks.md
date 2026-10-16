## 1. Planung und Tests
- [x] 1.1 ADR und Spec validieren; Gate 1 dokumentieren.
- [x] 1.2 Rote Szenarien in `internal/accounts/manager_test.go`, `internal/accounts/oauth_test.go`, `internal/accounts/gate_test.go`, `internal/cryptobox/box_test.go`, `web/src/accounts.test.ts` festhalten.
## 2. Umsetzung
- [x] 2.1 Tresor, Migration, PKCE und sichere Management-Schnittstelle implementieren.
- [x] 2.2 Wechsel, Sperre, Rollback, Audit und Beispielaufruf implementieren.
- [x] 2.3 Frontend hinzufügen, importieren, wechseln und Ergebnis anzeigen; Typen generieren.
## 3. Abschluss
- [x] 3.1 `make test`, `make lint`, unabhängiges Review und Belege.
- [x] 3.2 `just setup`, Container-Health und Beispielaufruf prüfen; Benutzer kann Account 2 hinzufügen.

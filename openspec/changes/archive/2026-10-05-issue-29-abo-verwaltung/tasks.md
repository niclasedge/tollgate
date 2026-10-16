## 1. Planung und Tests
- [x] 1.1 Bestand prüfen, Spec validieren, Gate 1 dokumentieren.
- [x] 1.2 Rote Tests: `internal/upstream/profile_test.go`, `internal/accounts/license_test.go`, `web/src/license.test.ts`, `web/src/accounts.test.ts`, `web/src/home.test.ts`, `web/src/account-usage.test.ts`; Ausgabe in Gate 2.
## 2. Umsetzung
- [x] 2.1 Profil-Read und Token-Zustand im Usage-Read, Persistenz im Messstand; Typen generieren.
- [x] 2.2 `AccountCard` mit Lizenzzeile, 5-Stunden-Balken, „Stand vom …“, Neu anmelden, Escape.
- [x] 2.3 Startseite mit Abo-Karten bei Account-Verwaltung.
## 3. Abschluss
- [x] 3.1 `make lint`, `make test`, `make build`, `openspec validate issue-29-abo-verwaltung --strict`.
- [x] 3.2 Screenshots desktop/iPhone × hell/dunkel mit erfundenen Daten nach `.claude/belege/issue29/`.

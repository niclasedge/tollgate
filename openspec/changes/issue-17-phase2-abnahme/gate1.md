# Gate 1

Branch `issue-17/phase2-abnahme`, gemeinsamer Workspace. Das Spec-Delta für `redaction` steht;
jedes Szenario hat einen geplanten Test in `cmd/tollgate/acceptance_test.go`:

| Szenario | Test |
|---|---|
| Vier Turns mit Rückübersetzung | `TestCacheStablePrefixAcrossTurns` |
| Konfigurationswechsel mitten in der Sitzung | `TestCacheStablePrefixAcrossTurns` (Versionen aus dem Trace-Detail, Sitzung B) |
| Leck-Suche | `TestNoPlaintextInDBLogsAndAdminAPI` |
| Fixtures Ende zu Ende | `TestEndToEndFixtures` |
| Neue Fixture ohne Eintrag | Pflichtprüfung in `runFixtures` |

Die manuelle Abnahme mit echtem Verkehr ist bewusst kein automatischer Test (keine echten
Accounts im Test); sie steht als offene Aufgabe 3.3 mit Anleitung im Phasenbericht.

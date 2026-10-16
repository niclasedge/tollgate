## Why
Nutzerauftrag ersetzt die bisherige Busy-Ablehnung: Account-Wechsel vormerken, laufende Anfragen abwarten, danach automatisch wechseln. Issue: https://github.com/niclasedge/tollgate/issues/88.
## What Changes
Gate reserviert Wechsel vor Drain, hält neue Requests zurück und weckt wartende Aufrufe nach Wechsel/Abbruch. Manager-Mutex wird erst nach Drain gehalten, damit begonnene Requests ihre Account-ID noch lesen können. Frontend zeigt wartenden Wechsel und Anzahl laufender Anfragen. Eine Vormerkung wartet auch auf einen laufenden Test/Wechsel; eine zweite gleichzeitig wartende Vormerkung wird eindeutig abgelehnt; Kontext-Abbruch vor Drain verändert keine Tokens. Das 80-Sekunden-Operationslimit beginnt nach dem Warten.
## Capabilities
### New Capabilities
- `queued-account-switch`: automatisch ausgeführter Wechsel nach Stream-Ende.
## Impact
Branch `issue-88/queued-account-switch`, gemeinsamer Workspace. Dateien: `internal/accounts/{gate,gate_test,types,manager,manager_test,queued_switch_test}.go`, `internal/server/{server,queued_switch_test}.go`, `internal/dashboard/accounts.go`, `web/src/lib/api.ts`, `web/src/lib/types/accounts.ts` (generiert), `web/src/lib/views/AccountManager.svelte`, `web/src/accounts.test.ts`, `docs/DEPLOY.md`, diese Artefakte. Historisches Busy-Szenario in #28 wird durch den aktuellen Nutzerauftrag ersetzt. Kein Abbruch laufender Streams, keine Client-Payload-Änderung.

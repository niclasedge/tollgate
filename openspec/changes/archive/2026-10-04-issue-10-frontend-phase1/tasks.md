# Tasks

## 1. Backend (Branch `issue-10/api`)

- [x] 1.1 Store: Traces mit Client-Name und Ersparnis listen, Kosten-Aggregation nach Tag/Client/Modell (`TestListTracesWithClientJoinsNameAndSavedTokens`, `TestCostByDay`, `TestListTracesQueryPlanNoTempSort`).
- [x] 1.2 `internal/dashboard`: Handler für traces, costs, status; Broker und SSE-Handler (`TestRoutesNeedAdminToken`, `TestTracesPagination`, `TestTracesParameters`, `TestCosts`, `TestStatus`, `TestStatusUpstreamTimesOutAfterThreeSeconds`, `TestBrokerNeverBlocksAndDropsForSlowSubscribers`, `TestEventsDeliverStartAndFinishOfProxiedRequest`, `TestEventsHeartbeat`, `TestHungSubscriberDoesNotDelayRequests`, `TestPublisherIsNoOpWithoutSubscribersAndResolvesNames`).
- [x] 1.3 Server: `/api/` an die Admin-Seite, Broker-Events bei Record/Finish (`TestRoutesAdminFrontendAndProxy`, `TestTraceObserverSeesStartAndFinish`).
- [x] 1.4 Config `upstream.management_key_file`, CPA-Management-Client, Wiring in `cmd/tollgate`, Deploy (Secret mounten, Entrypoint) (`TestManagementKeyFileFromYAMLAndEnv`, `TestWiringFrontendAdminAPIAndProxy`).

## 2. Frontend (Branch `issue-10/web`)

- [x] 2.1 Token-Abfrage, API-Client, fetch-basiertes SSE. Tests: `views.test.ts` (login: asks for the admin token, discards the token on 401, logs out), `api.test.ts` (Bearer header, 401, Fehlertext), `sse.test.ts` (createSseParser, createFetchEventSource inkl. Chunk-Grenzen).
- [x] 2.2 Live, Kosten, Clients, Accounts mit Tests (vitest): `views.test.ts` (Live, Kosten, Clients, Accounts), `format.test.ts`.

## 3. Prüfung

- [x] 3.1 `make lint`, `make test`, `make build` grün; `just smoke` grün.
- [x] 3.2 Belege: Screenshots der vier Bereiche mit echten Daten.
- [x] 3.3 PR mit `Closes #10`, CI grün, gemergt.

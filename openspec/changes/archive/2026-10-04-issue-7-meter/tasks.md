# Tasks

## 1. Umsetzung

- [x] 1.1 `internal/meter`: `FromJSON`, `Stream.Observe`/`Usage` (`TestFromJSON`, `TestStreamCollectsStartAndDelta`, `TestStreamDeltaUpdatesInputWhenPresent`, `TestStreamWithoutUsageEvents`, `TestEventDataJoinsLines`).
- [x] 1.2 Preise und `Cost` (`TestPriceFor`, `TestCost`).
- [x] 1.3 `sse.CopyObserve` (`TestCopyObserveSeesEveryEventUnchanged`).
- [x] 1.4 Server: Beobachter für `/v1/messages`, Usage in `FinishTrace`, Logzeilen (`TestUsageIsStoredForNonStreamingResponse`, `TestUsageIsStoredForStreamingResponse`, `TestCountTokensIsNotMetered`, `TestOversizedJSONResponseIsPassedThroughUnmetered`).
- [x] 1.5 Schätzheuristik im Code und in design.md begründet.
- [x] 1.6 Review-Nacharbeit: Vorfilter vor dem JSON-Decode je SSE-Event, Obergrenze 1<<40 je Usage-Wert gegen Überlauf, Modellname im Log gekürzt (`TestStreamHostileValues`, `FuzzMeterNeverPanics`, `TestEveryPriceKeyResolvesToItself`, `TestMeteredRequestIsLoggedWithNumbersOnly`, `TestAbortedStreamKeepsPartialUsage`, `TestUpstreamErrorBodyPassesUnchangedWithoutUsage`).

## 2. Prüfung

- [x] 2.1 `make lint`, `make test`, `make build` in frischer Kopie grün.
- [x] 2.2 `openspec validate issue-7-meter --strict` grün.
- [x] 2.3 PR #60 mit `Closes #7`, CI grün, gemergt.

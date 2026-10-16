# Gate 1
Branch `issue-91/negative-ersparnis`, gemeinsamer Workspace. Spec-Delta `frontend-views` legt Vorzeichen fest (positiv `−`, negativ `+`, 0/fehlend `–`). Geplanter Test: Tabellentest `formatSavedDelta` in `web/src/lib/format.test.ts` mit positiven, negativen, Tausender-, Null- und fehlenden Werten. Übersicht und Kosten zeigen keinen Präfix und bleiben unverändert.

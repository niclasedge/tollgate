# Proposal

Issue: [#41 Sitzungs-ID-Quelle an aufgezeichneter Anfrage verifizieren](https://github.com/niclasedge/tollgate/issues/41)

## Why

#5 leitet den Sitzungsschlüssel aus dem Header `X-Claude-Code-Session-Id`, sonst aus `metadata.user_id` ab. Beide Quellen und die zwei `user_id`-Formen waren nie an einer echten Anfrage geprüft. Stimmt die Annahme nicht, fallen alle Requests auf „Client ohne Sitzung“, und #13 kann die Konfiguration nicht pro Sitzung pinnen.

## What Changes

- Echte Anfrage von Claude Code 2.1.289 aufzeichnen, bereinigen und unter `testdata/fixtures/` ablegen (mit README zur Bereinigung).
- Intake-Test, der die Fixture lädt: nicht leerer Sitzungsschlüssel, Quelle `header`; ohne Header Quelle `metadata`.
- Kommentar „UNVERIFIED“ an `sessionFromUserID` durch die verifizierte Aussage ersetzen. Keine Verhaltensänderung.

## Capabilities

### New Capabilities

Keine.

### Modified Capabilities

- `pipeline-core`: Sitzungsschlüssel gegen aufgezeichnete Anfrage abgesichert.

## Impact

Nur Test, Fixture und ein Kommentar. Keine API-Änderung.

## Berührte Dateien

- `internal/pipeline/intake.go` (Kommentar), `internal/pipeline/intake_test.go`
- `testdata/fixtures/**`
- `openspec/changes/issue-41-session-fixture/**`

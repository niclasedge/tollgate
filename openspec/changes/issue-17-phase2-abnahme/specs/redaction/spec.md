## ADDED Requirements

### Requirement: Abnahme Cache-Präfix über das ganze Binary

Über `serve` mit eingeschalteter Redact-Stufe, echtem Store und versionierter Konfiguration SHALL jeder Turn einer Sitzung, der den Verlauf inklusive der rückübersetzten und vom Client im Klartext zurückgeschickten Antworten enthält, einen Upstream-Body erzeugen, dessen frühere Nachrichten Byte für Byte denen des vorigen Turns entsprechen. Die erneut redigierte Assistant-Nachricht MUST der Modellausgabe mit denselben Platzhalternummern entsprechen, sofern das Modell Platzhalter im Kontext ihres Fundes verwendet. Eine mitten in der Sitzung aktivierte Version MUST erst für neue Sitzungen gelten.

#### Scenario: Vier Turns mit Rückübersetzung
- **WHEN** eine Sitzung vier Turns schickt und das Modell jeweils die redigierte Nutzernachricht als Text, im `tool_use` und mit Platzhaltern im Thinking über Chunk-Grenzen zurückgibt
- **THEN** ist jeder Upstream-Body bis zum Ende der vorigen Nachrichten Byte-Präfix des nächsten, die Assistant-Nachricht upstream gleicht der Modellausgabe, und der Client sieht Klartext außer im Thinking.

#### Scenario: Konfigurationswechsel mitten in der Sitzung
- **WHEN** nach Turn 2 eine Version mit eigener Regel und Wortliste aktiviert wird
- **THEN** bleiben die Turns 3 und 4 auf Version 1 mit stabilem Präfix, und eine neue Sitzung redigiert mit Version 2.

### Requirement: Abnahme ohne Klartext in DB, Logs und Admin-API

Nach Cache-Sitzung und allen Fixtures MUST NOT ein erfundenes Geheimnis in den SQLite-Dateien samt WAL und SHM, in den Logs (Debug-Level), im Live-Stream `/api/traces/events` oder in einer Antwort von `/api/traces`, `/api/traces/{id}`, `/api/redactions`, `/api/costs`, `/api/status`, `/api/clients` und `/api/config/*` (ohne den ausdrücklichen Klartext-Export der Wortliste) stehen.

#### Scenario: Leck-Suche
- **WHEN** die Cache-Sitzung und alle Fixtures durchgelaufen sind
- **THEN** findet die Suche nach den Kanarienwerten in DB-Dateien, Logs, Live-Stream und Admin-API keinen Treffer.

### Requirement: Abnahme End-to-End mit bereinigten Fixtures

Jede Fixture unter `testdata/fixtures/` SHALL einen Eintrag mit ihren Geheimnissen und Allowlist-Werten im Abnahmetest haben und über das ganze Binary laufen: Der Upstream sieht nur Platzhalter, Allowlist-Werte, Thinking und Signaturen bleiben bytegleich, die SSE-Antwort mit Platzhaltern über Chunk-Grenzen erreicht den Client im Klartext. Eine Fixture ohne Geheimnisse MUST bytegleich durchgehen.

#### Scenario: Fixtures Ende zu Ende
- **WHEN** jede Fixture mit aktiver Konfiguration inklusive Wortliste gesendet wird
- **THEN** enthält kein Upstream-Body ein Geheimnis der Fixture, und der Client erhält das Zitat der letzten Nutzernachricht im Klartext in Text und Tool-Input.

#### Scenario: Neue Fixture ohne Eintrag
- **WHEN** eine Fixture-Datei existiert, für die der Test keinen Eintrag hat, oder ein Eintrag keine Datei hat
- **THEN** scheitert der Test.

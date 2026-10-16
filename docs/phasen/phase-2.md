# Phasenbericht Phase 2: Redactor

Stand: 2026-10-05, Abnahme-Change `issue-17-phase2-abnahme` (#17).

## Was gebaut ist

| Issue | Ergebnis |
|---|---|
| #12 | Go-Port des Rust-Redactors (`internal/redact`): 19 Regeln, False-Positive-Filter, „erst sammeln, dann auflösen“, Korpus unter `internal/redact/testdata/corpus/`, Paritätstest gegen das Rust-Repo (ADR-0005). |
| #13 | Redact-Stufe: nummerierte Platzhalter `<redacted:TYP_N>` pro Anfrage in Dokumentreihenfolge, nur die Blöcke aus ADR-0006 (auch Assistant-Text und `tool_use.input`), Zuordnung nur im RAM, Sitzungs-Pinning der Konfigurationsversion, `count_tokens` mit redigiert, Schalter `redact.enabled`. |
| #14 | Rückweg: Platzhalter in `text_delta` und `input_json_delta` zurückübersetzt, Puffer pro Content-Block über Chunk-Grenzen, Thinking und Signaturen unverändert, Nicht-Streaming per Byte-Splicing. |
| #15 | Versionierte Konfiguration (`internal/redactcfg`): JSON-Schema, YAML-Import und -Export, Dry-Run auf `testdata/fixtures/`, Versionen mit Diff und Rollback, Hot Reload, eigene Regex, verschlüsselte Wortliste, Allowlist, Admin-API `/api/config/*`. |
| #16 | Frontend: Filter-Bereich (Formular und YAML-Editor, Live-Test, Versionen, Import/Export), Stufen-Timeline mit Diff in Live, „Letzte Redactions“ in der Übersicht; Platzhalter in gespeicherten Treffern. |
| #83 | Rest eines verdrängten generischen Treffers wird mit redigiert statt im Klartext stehen zu bleiben. |
| #84 | `client_secret` endet am schließenden Anführungszeichen; Vorfilter je Regel für Leistung, ohne Treffer zu verlieren. |

## Wie getestet

Pakettests (tabellengetrieben, Golden-Files, Fuzz, Parität, `httptest`-Integration) stammen
aus den einzelnen Changes. Die Abnahme #17 ergänzt drei Tests über das ganze Binary
(`cmd/tollgate/acceptance_test.go`): `serve` mit Redact-Stufe, echtem SQLite-Store,
versionierter Konfiguration und Debug-Logs; CLIProxyAPI ist ein Mock-Modell, das die
redigierte letzte Nutzernachricht als SSE zitiert (Thinking mit Platzhaltern, Text,
`tool_use`, Deltas zu 7 Bytes, jedes Event in zwei Writes).

| Test | Nachweis |
|---|---|
| `TestCacheStablePrefixAcrossTurns` | Vier Turns einer Sitzung, Claude Code schickt die rückübersetzten Antworten im Klartext zurück. Jeder Upstream-Body ist bis zum Ende der vorigen Nachrichten Byte-Präfix des nächsten; die erneut redigierte Assistant-Nachricht gleicht der Modellausgabe; `email_1`/`api_key_1` bleiben, neue Werte bekommen höhere Nummern. Nach Turn 2 wird eine Version mit eigener Regel und Wortliste aktiviert: Sitzung A bleibt auf Version 1, eine neue Sitzung B nutzt Version 2. |
| `TestEndToEndFixtures` | Jede Fixture aus `testdata/fixtures/` (neu: `multi-turn-session.json`): Upstream nur mit Platzhaltern, Allowlist-Wert, Thinking und Signaturen bytegleich, Fixture ohne Geheimnis bytegleich, Client bekommt Text und Tool-Input im Klartext. |
| `TestNoPlaintextInDBLogsAndAdminAPI` | Nach Cache-Sitzung und Fixtures kein Kanarienwert in `t.db`, WAL, SHM, Debug-Logs, Live-Stream `/api/traces/events` und den Antworten von `/api/traces`, `/api/traces/{id}`, `/api/redactions`, `/api/costs`, `/api/status`, `/api/clients`, `/api/config/*`. |

`make lint`, `make test` (`go test -race ./...` plus Frontend) und `make build` sind grün.

## Was offen ist

1. **Manuelle Abnahme mit echtem Verkehr** (offen, Niclas): „Claude Code arbeitet normal über
   tollgate, Cache-Trefferquote nicht schlechter als ohne tollgate.“ Anleitung unten. Nicht
   automatisiert, weil sie echte Accounts braucht.
2. **Platzhalter ohne Kontext** (#97, siehe
   `openspec/changes/issue-17-phase2-abnahme/design.md`, Abschnitt Befund): Schreibt das
   Modell einen Platzhalter einer Kontextregel ohne seinen Kontext (z. B.
   `<redacted:connection_password_1>` statt der ganzen URL), bekommt Claude Code den Klartext
   und schickt ihn im nächsten Turn als Assistant-Text zurück. Der Detektor erkennt den Wert
   ohne Kontext nicht: Der Klartext geht upstream und landet in der Diff-Umgebung naher
   Platzhalter. Die Byte-Präfixe bleiben stabil.
3. **#90** `redact.enabled` standardmäßig einschalten (bisher aus).
4. **#91** Anzeige negativer Ersparnis in Live.

## Anleitung: manuelle Abnahme mit echtem Verkehr

Ziel: Gleiche Arbeit einmal ohne und einmal mit Redaktion, Kennzahlen im Frontend vergleichen.
Die Referenz „ohne tollgate“ ist tollgate mit `redact.enabled: false`: Der Body geht dann
bytegleich durch (Phase 1), gemessen wird aber am selben Ort.

**Vorbereitung**

1. tollgate und CLIProxyAPI laufen laut `docs/DEPLOY.md`, ein Account ist aktiv (Bereich
   Accounts).
2. Im Bereich Clients zwei Clients anlegen: `abnahme-aus` und `abnahme-mit`. Die Kosten
   lassen sich so pro Client trennen.
3. Ein leeres Testverzeichnis mit einer Datei `notizen.txt` und nur erfundenen Werten, z. B.
   `Kontakt: jane.roe@muster-gmbh.invalid`, `DB: postgres://app:fakepw-0000@db.invalid:5432/app`,
   `Projekt Falke`. Keine echten Geheimnisse.
4. Im Bereich Filter eine Version mit `Projekt Falke` in der Wortliste aktivieren.

**Lauf A (ohne Redaktion)**

1. `TOLLGATE_REDACT_ENABLED=false` setzen, tollgate neu starten.
2. Im Testverzeichnis: `ANTHROPIC_BASE_URL` auf tollgate, `ANTHROPIC_AUTH_TOKEN` auf den Key
   von `abnahme-aus`, `claude` in einer neuen Sitzung starten.
3. Feste Folge von sechs Prompts, gleiche Reihenfolge, gleiches Modell:
   1. „Lies notizen.txt und fasse sie zusammen.“
   2. „Schreib die Kontaktadresse in eine neue Datei kontakt.txt.“
   3. „Führe `cat kontakt.txt` und `wc -l notizen.txt` aus.“
   4. „Wie lautet das Datenbank-Passwort? Nenne nur das Passwort.“
   5. „Ersetze in notizen.txt Projekt Falke durch Projekt Adler.“
   6. „Zeig `git diff --no-index /dev/null notizen.txt` oder den Dateiinhalt.“
4. Sitzung beenden.

**Lauf B (mit Redaktion)**

1. `TOLLGATE_REDACT_ENABLED=true`, tollgate neu starten, Testverzeichnis zurücksetzen.
2. Gleiche Folge mit dem Key von `abnahme-mit` in einer neuen Sitzung.

**Vergleichen**

| Wo | Kennzahl | Erwartung |
|---|---|---|
| Kosten, Tabelle „Pro Client“, Spalte Cache | Cache-Trefferquote = `cache_read / (input + cache_read + cache_creation)` | B ist nicht schlechter als A (Toleranz 2 Prozentpunkte bei gleicher Zahl an Anfragen). |
| Live, Detail jeder Anfrage | Cache gelesen / Cache angelegt | Ab der zweiten Anfrage hat B wie A `cache_read > 0`; `cache_creation` deckt nur den neuen Turn ab, kein Sprung auf die ganze Länge. |
| Live, Stufen-Timeline | Diff der Redact-Stufe, Konfigurationsversion | Platzhalter statt Werten; alle Anfragen der Sitzung tragen dieselbe Version. |
| Übersicht, Letzte Redactions | Typ und Platzhalter | `email`, `connection_password`, `word`, keine Werte. |
| Claude Code selbst | Verhalten | Werkzeuge laufen, Dateien enthalten die echten Werte, keine Platzhalter außer im Thinking, keine API-Fehler (z. B. Signatur). |

Prompt 4 provoziert den offenen Punkt 2: Nennt das Modell nur den Platzhalter, steht das
Passwort im nächsten Turn im Klartext in der Upstream-Anfrage. Das ist bekannt und kein
Abbruchgrund für die Cache-Abnahme, wird aber im Ergebnis notiert.

**Ergebnis festhalten**: Trefferquoten A und B, Zahl der Anfragen, Auffälligkeiten als Kommentar
in #17. Erst danach ist Phase 2 abgenommen.

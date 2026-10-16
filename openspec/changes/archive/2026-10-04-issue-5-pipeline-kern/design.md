# Design

## Context

ADR-0004 verlangt, dass der Request-Body nie neu serialisiert wird: Stufen liefern Ersetzungslisten, `jsonsplice.Apply` setzt die Ausgabe aus den Originalbytes zusammen. ADR-0006 legt fest, welche Blöcke bearbeitet werden dürfen und wie der Sitzungsschlüssel entsteht. ADR-0003 fixiert `github.com/go-json-experiment/json` (Paket `jsontext`) als Tokenizer, per Pseudo-Version gepinnt (`v0.0.0-20260820222146-c27c302e5fc3`, `go get …@master` am 2026-10-04).

## Goals / Non-Goals

**Goals:** verlustfreier Roundtrip, Ersetzungslisten als Schnittstelle zwischen Stufen, vollständiger Trace pro Stufe, Eingangsstufe mit Auth-Abstraktion.
**Non-Goals:** HTTP, Key-Verwaltung, Datenbank, Redact/Compress, Meter-Heuristik.

## Decisions

### jsonsplice

- **Scan** liest den Body einmal mit `jsontext.Decoder` (strikt: RFC 8259, ungültiges UTF-8 und doppelte Member-Namen werden abgelehnt, keine Daten nach dem Wert). Doppelte Namen werden bewusst abgelehnt: Ein mehrdeutiges Objekt könnte Redaktion umgehen (zwei `messages`); Anthropic würde sie ohnehin nicht sinnvoll verarbeiten.
- **Ziele** (Blocktabelle ADR-0006): `messages[i].content` als String, `text` von `text`-Blöcken, `tool_result.content` (String oder `text`-Blöcke im Array), alle String-Werte rekursiv in `tool_use.input` (nur Werte, keine Schlüssel). Rolle (`user`/`assistant`) wird mitgeliefert; die Stufe entscheidet.
- **Typ nach Feld:** Weil `type` nach den Feldern stehen kann, werden Kandidaten gesammelt und erst nach dem Blockende nach Typ gefiltert. `thinking`, `redacted_thinking`, `compaction`, `image`, `document` und unbekannte Typen liefern nie Ziele. `system`, `tools`, `metadata` und unbekannte Top-Level-Felder werden nur übersprungen.
- **Range = ganzes String-Token inkl. Anführungszeichen.** `Apply` schreibt `"` + `Escape(neu)` + `"`. `Apply` prüft, dass jeder Bereich genau ein gültiges String-Token ist, Bereiche sich nicht überlappen und `New` gültiges UTF-8 ist; sonst Fehler. So ist die Ausgabe nach erfolgreichem `Apply` gültiges JSON.
- **Vorwärts statt rückwärts:** `Apply` sortiert stabil nach Start und baut in einem Durchgang vorwärts auf; das ist gleichwertig zum Rückwärts-Splicing aus ADR-0004 und braucht keine Offset-Korrektur.
- **Escape** exakt nach ADR-0004 Punkt 4 (`"`, `\`, U+0000–U+001F; U+007F und Nicht-ASCII roh, `\u00xx` kleingeschrieben).
- Zusätzlich liefert `Scan` `model`, `stream` und `metadata.user_id` (Top-Level-Strings/Bool), damit Stufe 0 ohne zweiten Durchlauf auskommt.

### pipeline

- **Stage:** `Process(ctx, *Request) ([]jsonsplice.Replacement, error)`. Der Runner wendet die Liste an; die nächste Stufe sieht den neuen Body. Offsets gelten nur für den Body, den die Stufe erhalten hat, darum scannt jede Stufe selbst und `Request` enthält keine Offsets.
- **Trace pro Stufe:** Name, Dauer, geschätzte Tokens vorher/nachher, SHA-256 hex von Ein- und Ausgabe, Konfigurationsversion, Ersetzungsliste (Diff laut ADR-0004). Die Liste enthält nur die neuen (redigierten) Werte, nie die ersetzten Originale. Fehlerstufen sind mit `Failed` markiert; Fehlertexte von Stufen kommen nicht in Trace oder Client-Antwort (`StageError.Cause` ist nur intern).
- **Trace-ID:** 16 Byte `crypto/rand`, 32 Hex-Zeichen, vor Stufe 0 vergeben, damit auch abgelehnte Anfragen eine ID haben. Zeit und ID erscheinen nur im Trace, nie im weitergeleiteten Body; Uhr und ID-Generator sind per Option ersetzbar.
- **Tokens:** `Estimator`-Funktion, Standard `ceil(bytes/4)` als Platzhalter bis #7 die Heuristik festlegt.
- **Fehler:** `StageError` mit Status/Typ/fester Meldung; `ErrorResponse` baut den Anthropic-Fehlerkörper byte-stabil (401 `authentication_error`, 400 `invalid_request_error`, 500 `api_error`). Ein Stage-Fehler oder ungültige Ersetzungen brechen ab; es wird nicht weitergeleitet (ADR-0004 Punkt 7).
- **Intake (Stufe 0):** Key aus `Authorization: Bearer …` (Schema case-insensitiv), sonst `x-api-key`. `KeyVerifier.Verify` liefert Client-ID oder `ErrInvalidKey` (→ 401); andere Fehler → 500. Auth gilt für alle Pfade, geparst wird nur `POST /v1/messages` und `POST /v1/messages/count_tokens`; alles andere ist `EndpointPassthrough` und bleibt unangetastet. Konstantzeit-Vergleich und Hashing gehören zur Implementierung in #8.
- **Sitzungsschlüssel:** `hex(SHA-256(client_id ‖ 0x00 ‖ session_id))`. Das Trennbyte 0x00 verhindert Mehrdeutigkeit der Konkatenation (festgehalten in ADR-0011, weil ADR-0006 nur `‖` schreibt). Ohne Sitzungs-ID bleibt der Schlüssel leer („Client ohne Sitzung“). Im Trace steht nur der Hash.
- **Konfigurationsversion:** optionales `ConfigPinner`-Interface; ohne Pinner bleibt die Version 0. Die echte Implementierung (Tabelle `sessions`) folgt mit #4/#13.

## Offene Punkte / Annahmen

- **Quelle der Sitzungs-ID: NICHT an einer aufgezeichneten Anfrage verifiziert.** `testdata/fixtures/` enthält keine bereinigte Fixture. Umgesetzt ist die Reihenfolge aus ADR-0006: Header `X-Claude-Code-Session-Id`, sonst Session-Anteil von `metadata.user_id`. Für `user_id` werden zwei Formen angenommen, beide ungeprüft: JSON-Objekt-String mit `session_id` und das ältere `user_<hash>_account_<uuid>_session_<uuid>`. Die Verifikation an einer echten, bereinigten Claude-Code-2.1.x-Anfrage bleibt offen (ausgelagert nach #41, blockiert #13).
- Standard-Tokenschätzung ist ein Platzhalter bis #7.
- Der Fuzz-Test läuft in `make test` nur über seine Seeds; ein längerer Lauf ist manuell (`go test -fuzz FuzzApply ./internal/jsonsplice`).

# json-splicing Specification

## Purpose
`jsonsplice` ermöglicht Stufen, ausgewählte Strings eines Request-Bodys zu ersetzen, ohne dass sich andere Bytes ändern.

## Requirements

### Requirement: Verlustfreier Roundtrip

`jsonsplice.Apply` SHALL ohne Ersetzungen den Body bytegleich zurückgeben, und mit Ersetzungen MUST alle Bytes außerhalb der ersetzten String-Token unverändert bleiben, auch bei abweichendem Whitespace, Escaping, Zahlenformen, Feldreihenfolge, unbekannten Feldern und unbekannten Content-Typen.

#### Scenario: Keine Ersetzung
- **WHEN** ein Body mit Sonderzeichen, Zahlen wie `1.0E3` und unbekannten Feldern gescannt und ohne Ersetzungen angewendet wird
- **THEN** ist die Ausgabe bytegleich zur Eingabe.

#### Scenario: Zwei Ersetzungen in beliebiger Reihenfolge
- **WHEN** zwei Ziele in umgekehrter Reihenfolge ersetzt werden
- **THEN** unterscheidet sich die Ausgabe nur in den beiden Token und ist gültiges JSON.

### Requirement: Zielauswahl nach Blocktabelle

`jsonsplice.Scan` SHALL nur Ziele liefern, die ADR-0006 erlaubt: String-Content von Nachrichten, `text`-Blöcke, `tool_result`-Inhalt und String-Werte in `tool_use.input`. `thinking`, `redacted_thinking`, `signature`, `compaction`, `system`, `tools`, `metadata`, `image`, `document` und unbekannte Blocktypen MUST NOT Ziele liefern, unabhängig von der Reihenfolge der Felder im Block.

#### Scenario: Gesperrte und unbekannte Blöcke
- **WHEN** ein Body mit allen gesperrten Blocktypen und einem unbekannten Typ mit `text`-Feld gescannt wird
- **THEN** erscheint für keinen davon ein Ziel.

#### Scenario: Typ nach dem Feld
- **WHEN** ein `text`-Block sein `type` nach dem `text`-Feld nennt
- **THEN** wird der Text als Ziel geliefert.

### Requirement: Strenge Eingabeprüfung

`Scan` SHALL ungültiges JSON, ungültiges UTF-8, doppelte Member-Namen, Nicht-Objekte auf oberster Ebene, ein `messages`-Feld ohne Array und Daten nach dem Wert mit einem Fehler ablehnen. `Apply` SHALL Bereiche ablehnen, die kein einzelnes String-Token sind, sich überlappen oder außerhalb liegen, sowie Ersatzwerte mit ungültigem UTF-8.

#### Scenario: Abgeschnittener Body
- **WHEN** ein abgeschnittener Body gescannt wird
- **THEN** liefert `Scan` einen Fehler.

### Requirement: Feste Escape-Funktion

`jsonsplice.Escape` SHALL nur `"`, `\` und U+0000–U+001F escapen (`\n`, `\t`, `\r`, `\b`, `\f`, sonst `\u00xx`) und alle anderen Bytes roh lassen.

#### Scenario: Nicht-ASCII
- **WHEN** `ä € 😀` escaped wird
- **THEN** bleibt der Text unverändert.

### Requirement: Fuzz-geprüfte Invarianten

Das Paket SHALL einen Fuzz-Test (`testing.F`) enthalten, der prüft: keine Ersetzung ergibt identische Bytes, jede Ersetzung ergibt gültiges JSON mit dem neuen Wert als dekodiertem Zielstring, und die Bytes zwischen den Zielen bleiben identisch.

#### Scenario: Fuzzing
- **WHEN** `go test -fuzz FuzzApply ./internal/jsonsplice` läuft
- **THEN** wird keine Invariante verletzt.

# ADR-0011: Sitzungsschlüssel mit Trennbyte

- **Status:** Angenommen
- **Datum:** 2026-10-04
- **Issue:** #5

## Kontext

[ADR-0006](0006-platzhalter-sitzungen-rueckweg.md) definiert den Sitzungsschlüssel als
`SHA-256(client_id ‖ session_id)`, legt aber weder ein Trennzeichen noch die Kodierung fest.
Ohne Trennzeichen sind die Paare `("ab", "c")` und `("a", "bc")` nicht unterscheidbar und
ergeben denselben Schlüssel. Der Schlüssel wird an zwei Stellen berechnet bzw. gespeichert:
im Eingang der Pipeline (#5) und in der Tabelle `sessions` (#4, #13). Beide müssen exakt
dieselbe Formel nutzen, sonst geht das Pinning der Konfigurationsversion still verloren.

## Entscheidung

Der Sitzungsschlüssel ist `hex(SHA-256(client_id ‖ 0x00 ‖ session_id))`:

- `0x00` ist ein einzelnes Nullbyte zwischen den UTF-8-Bytes beider Werte. Weder
  Client-IDs noch Sitzungs-IDs enthalten ein Nullbyte.
- Der Hash wird als 64 Zeichen Kleinbuchstaben-Hex gespeichert und übertragen.
- Ohne Sitzungs-ID gibt es keinen Schlüssel (leerer String, „Client ohne Sitzung“ nach
  ADR-0006).

Alles Übrige aus ADR-0006 zur Sitzung (Quellen der Sitzungs-ID, Pinning) gilt unverändert.
Die Quelle der Sitzungs-ID wird in #41 an einer aufgezeichneten Anfrage verifiziert.

## Alternativen

- **Konkatenation ohne Trenner (Wortlaut von ADR-0006):** mehrdeutig, verworfen.
- **Längenpräfix oder JSON-Kodierung des Paars:** ebenfalls eindeutig, aber aufwendiger
  nachzurechnen. Das Nullbyte reicht, weil es in keinem der Werte vorkommen kann.

## Konsequenzen

- Pipeline-Eingang, Store und Tests verwenden diese Formel. Abweichende Implementierungen
  fallen im Test `TestSessionKeyDerivation` (`internal/pipeline`) auf.
- `docs/ARCHITECTURE.md` nennt die Formel mit Trennbyte.

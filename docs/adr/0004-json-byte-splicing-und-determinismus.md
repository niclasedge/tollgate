# ADR-0004: JSON per Byte-Splicing und Determinismus

- **Status:** Angenommen (Freigabe Niclas, 2026-10-04, PR #26)
- **Datum:** 2026-10-04
- **Issue:** #1

## Kontext

Zwei harte Regeln aus `CLAUDE.md` betreffen jede Transformation:

1. *„Alle Transformationen sind deterministisch: gleiche Eingabe, Byte für Byte gleiche
   Ausgabe. Keine Zeitstempel, kein Zufall, keine Map-Iterationsreihenfolge. Sonst bricht das
   Prompt-Caching.“*
2. *„Unbekannte Content-Typen und Felder werden unverändert durchgereicht. JSON wird so
   verarbeitet, dass nicht modellierte Felder nicht verloren gehen.“*

Dazu kommt: *„Thinking-Blöcke und ihre Signaturen werden nie verändert.“*

Wer einen `/v1/messages`-Body in Go-Structs oder `map[string]any` parst und neu serialisiert,
verliert dabei Bytes:
- Feldreihenfolge
- Whitespace
- Escaping (`ä` vs. `ä`)
- Zahlendarstellung
- unbekannte Felder

Schon ein geändertes Escaping in einem früheren Turn ändert den Präfix und bricht den Cache.

Headroom löst das Problem in Rust mit Ersetzungen auf Byte-Bereichen von `RawValue`-Slices.
Der Test `byte_fidelity_outside_compressed_block` hält das fest
(`crates/headroom-core/src/transforms/live_zone.rs`,
`crates/headroom-core/tests/live_zone_dispatch.rs`).

## Entscheidung

### Byte-Splicing

1. **Body nie neu serialisieren.** Der Request-Body bleibt als `[]byte` erhalten.
2. **Zielstrings über Offsets finden.** Ein Tokenizer (`jsontext`, ADR-0003) läuft einmal
   über den Body und liefert die Byte-Offsets der Zielstrings. Dabei verfolgt er den Pfad:
   `messages[i].role`, `messages[i].content[j].type` und das jeweilige Zielfeld (`text`,
   `content`, String-Werte in `input`).
3. **Gezielt ersetzen.** Jede Stufe erzeugt eine Liste von Ersetzungen
   `{start, end, neuer_string}`. `jsonsplice.Apply` setzt die Ausgabe zusammen:
   `original[:start] + escape(neu) + original[end:]`, von hinten nach vorn, damit die Offsets
   gültig bleiben. Alle anderen Bytes sind identisch.
4. **Eigene, feste Escape-Funktion.** Neu kodierte Strings escapen nur `"`, `\` und
   Steuerzeichen U+0000–U+001F (`\n`, `\t`, `\r`, `\b`, `\f`, sonst `\u00XX`). Alles andere,
   auch Nicht-ASCII-UTF-8, bleibt roh. Strings, die nicht ersetzt werden, behalten ihr
   Original-Escaping.
5. **Unbekanntes bleibt unberührt.** Unbekannte Content-Typen und Felder werden nicht
   besucht und deshalb nicht verändert. Das erfüllt die zweite harte Regel strukturell, ohne
   jedes Feld modellieren zu müssen.
6. **Gesperrte Blocktypen.** `thinking`, `redacted_thinking`, `signature`, `compaction` und
   `system` erzeugen nie Ersetzungen. Ein Test prüft für einen Body mit allen diesen
   Blocktypen, dass sie bytegleich bleiben.
7. **Ungültiges JSON oder Fehler in einer Stufe: Abbruch.** Ist der Body kein gültiges JSON
   oder schlägt eine Stufe fehl, antwortet tollgate mit `400 invalid_request_error` bzw.
   `500 api_error` im Anthropic-Fehlerformat. Die Anfrage wird **nicht** weitergeleitet.
   Unverändertes Weiterleiten könnte Klartext-Geheimnisse upstream schicken. Ungültiges
   JSON würde Anthropic ohnehin ablehnen.

### Determinismus-Checkliste

Gilt für jede Stufe und wird in Code-Reviews geprüft:

- [ ] Ausgabe hängt nur von Eingabe-Bytes und gepinnter Konfigurationsversion ab.
- [ ] Kein `time.Now()` und kein Zufall im Transformationspfad. Trace-IDs und Zeitstempel
      landen nur in Traces, nie im weitergeleiteten Body.
- [ ] Keine Iteration über Go-Maps, die die Ausgabe beeinflusst. Wenn nötig, Schlüssel
      vorher sortieren.
- [ ] Sortierungen sind stabil (`slices.SortStableFunc`) und vollständig definiert, mit
      Tie-Breaker.
- [ ] Keine Abhängigkeit von Umgebung (Locale, Zeitzone, Hostname) oder Nebenläufigkeit.
- [ ] Regex nur über `regexp` (RE2, linear, leftmost-first).

### Prüfung

- Fuzz-Test (`testing.F`) für `jsonsplice`: keine Ersetzungen ergibt eine bytegleiche
  Ausgabe. Jede Ersetzung ergibt gültiges JSON, dessen Zielstring dekodiert genau dem neuen
  Wert entspricht.
- Determinismus-Test pro Stufe: 100 Läufe über dieselbe Eingabe, auch parallel mit `-race`,
  ergeben einen identischen SHA-256.
- Byte-Treue-Test: Alle Bytes außerhalb der Ersetzungsbereiche sind identisch, gemessen per
  SHA-256 der Bereiche vor, zwischen und nach den Ersetzungen.

## Alternativen

- **Structs mit `json.RawMessage` für unbekannte Teile:** verworfen. Die Hüllobjekte werden
  trotzdem neu serialisiert, Feldreihenfolge und Escaping ändern sich.
- **`map[string]any`:** verworfen. Die Reihenfolge geht verloren, und Zahlen werden zu
  `float64`.
- **gjson/sjson:** verworfen. `sjson.SetRaw` arbeitet zwar ähnlich, aber pfadbasiert pro
  Aufruf, also bei vielen Ersetzungen quadratisch. Außerdem ist der Parser weniger strikt.

## Konsequenzen

- Alle Stufen arbeiten mit Ersetzungslisten statt mit Objektmodellen. Das macht den Diff für
  die Timeline im Frontend trivial: Die Ersetzungsliste *ist* der Diff.
- **#5** implementiert `jsonsplice` und den Pfad-Tokenizer, **#13** und **#19** nutzen ihn.
- Rehydrierte Werte im Rückweg (ADR-0006) laufen durch dieselbe Escape-Funktion.

## Quellen

- `CLAUDE.md`, Abschnitt „Harte Regeln“
- Headroom: `crates/headroom-core/src/transforms/live_zone.rs`,
  `crates/headroom-core/tests/live_zone_dispatch.rs`, `REALIGNMENT/02-architecture.md`
  (Invarianten I1–I8)

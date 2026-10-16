## Kontext
Live zeigt die Ersparnis als Token-Delta der Anfrage: „−12“ heißt 12 Tokens weniger. Das Vorzeichen wird bisher als Literal vor `formatTokens(saved)` gesetzt; bei negativem Wert liefert `Intl.NumberFormat` ein eigenes Minus.
## Entscheidung
- `formatSavedDelta(saved)` formatiert den Betrag (`Math.abs`) und wählt das Vorzeichen selbst: `saved > 0` → `−`, `saved < 0` → `+`. So steht immer genau ein Vorzeichen da.
- Negative Ersparnis als „+12“ statt „−12“: Das Issue lässt beides zu; „−12“ wäre von 12 gesparten Tokens nicht zu unterscheiden. Der Tooltip „Gesparte Tokens“ bleibt; ein „+“ ist als Mehrverbrauch lesbar.
- 0, `null` und `undefined` ergeben wie bisher „–“.
- Übersicht und Kosten bleiben unverändert: sie zeigen den Wert ohne Präfix, ein negativer Wert hat dort genau ein Minus.
## Risiken
Keine; reine Darstellung, deterministisch, kein Einfluss auf Pipeline oder Cache.

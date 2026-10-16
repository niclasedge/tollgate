# Design

Verbindlich ist ADR-0005. Hier steht, wie `Find` die Restspannen bildet.

## Ausgangslage

`Find` sortiert die Kandidaten nach Priorität, Länge, Start und Regelreihenfolge und
übernimmt jeden, der kein bereits übernommenes Byte berührt. Ein überlappender Kandidat
fällt ganz weg. Bei generischen Regeln, deren Secret-Gruppe weit greift (`auth_header`:
`\S.*` bis zum Zeilenende), bleibt so der Rest des Werts stehen.

## Entscheidung

Beim Verwerfen eines Kandidaten mit Priorität `prioGeneric` werden die nicht abgedeckten,
maximalen Läufe seiner Secret-Spanne betrachtet. Zu diesem Zeitpunkt sind alle Treffer
höherer Priorität und alle längeren generischen Treffer schon übernommen; die Reihenfolge
bleibt die bestehende Sortierung, also deterministisch.

Jeder Lauf wird

1. an beiden Rändern um Unicode-Whitespace gekürzt,
2. verworfen, wenn er keinen Buchstaben und keine Ziffer enthält (`unicode.IsLetter`,
   `unicode.IsDigit`) – schließende Anführungszeichen oder Klammern sind kein Geheimnis,
3. verworfen, wenn er ganz im Nicht-Secret-Teil eines anderen Kandidaten liegt (Teil des
   Regex-Treffers vor oder nach dessen Secret-Gruppe). So bleiben das Schema `Bearer`
   (Präfix von `bearer_token`) und ein Schlüsselname wie `password=` (Präfix von
   `generic_assign`) lesbar – genau die Texte, die ein anderer Detektor ausdrücklich als
   Nicht-Geheimnis einordnet,
4. verworfen, wenn er ein Authentifizierungsschema `Bearer`, `Basic` oder `Digest` ist
   (Groß-/Kleinschreibung egal). Das sind die Schemata, die `bearer_token` als Präfix liest;
   `bearer_token` trifft aber erst ab 16 Zeichen Token, bei kurzen Tokens fehlt sein
   Kandidat und Regel 3 greift nicht,
5. verworfen, wenn sein Wert in der Allowlist steht (wie jeder andere Treffer).

Die übrigen Läufe werden sofort als Treffer mit Regel, Typ, Priorität und Reihenfolge des
verdrängten Kandidaten übernommen und als abgedeckt markiert. Spätere, kürzere Kandidaten
sehen sie wie jeden anderen übernommenen Treffer.

Beispiel `Authorization: Bearer eyJ….sig weiterer-text`: `jwt` (spezifisch) wird übernommen,
`bearer_token` hat dieselbe Spanne und fällt weg, `auth_header` (Wert
`Bearer eyJ….sig weiterer-text`) wird verdrängt. Läufe: `Bearer ` → `Bearer` liegt im Präfix
von `bearer_token` → bleibt; ` weiterer-text` → `weiterer-text` → Treffer `auth_header`.
Ausgabe: `Authorization: Bearer <redacted:jwt> <redacted:api_key>`.

## Warum nur generische Treffer

Bei spezifischen Regeln ist der Rest meist bewusst kein Geheimnis: `conn_string` verdrängt
`email` in `app:pass@db.prod.io`, der Rest `@db.prod.io` ist der Host. Der Auftrag aus #83
nennt ausdrücklich generische Treffer.

## Alternativen

- **Generischen Treffer gewinnen lassen:** verliert das spezifische Label (ADR-0005 verwirft
  das ausdrücklich).
- **Nur feste Liste von Schemata (`Bearer|Basic|Digest`) ohne Regel 3:** deckt `password=`
  und andere Schlüsselnamen nicht ab. Regel 3 nutzt die vorhandene Einordnung der
  Detektoren, Regel 4 schließt nur die Lücke kurzer Tokens.
- **Nur Regel 3 ohne Schemata:** erster Entwurf; bei `Authorization: Bearer eyJa.eyJb.c`
  (Token unter 16 Zeichen) wurde `Bearer` ersetzt (Gate 2).
- **`auth_header` mit eigener Schema-Gruppe:** würde auch die Ausgabe für undurchsichtige
  Tokens ändern (`Authorization: Bearer <redacted>` statt `Authorization: <redacted>`), das
  ist nicht Teil von #83.
- **Mindestlänge für Restspannen:** willkürlich; ein kurzer Rest kann trotzdem geheim sein.
  Regel 2 filtert nur reine Satzzeichen.

## Parität

Rust überschreibt den ganzen Header-Wert mit `auth_header`; Go lässt das Schema und das
spezifische Label stehen und ersetzt den Rest gesondert. Das bleibt die bewusste Abweichung
`overlap` (ADR-0005 Paritätstest, Punkt 1), der Nachtrag in ADR-0005 beschreibt die
Restspannen. Die bisherigen `overlap`-Fälle ändern ihre Go-Ausgabe nicht.

## Risiken

- Zusätzliche Treffer verschieben die Nummerierung späterer Platzhalter nur in Anfragen, die
  solche Reste enthalten. Bestehende Sitzungen behalten ihre Konfigurationsversion, die
  Regel selbst ist aber Code: Nach einem Update kann sich ein Präfix mit solchem Rest einmal
  ändern (einmaliger Cache-Miss, wie bei jedem Redactor-Update).

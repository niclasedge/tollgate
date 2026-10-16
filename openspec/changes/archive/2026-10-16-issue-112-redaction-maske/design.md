## Kontext
Die Redact-Stufe (`redact.Number`) kennt je Treffer Regel, Typ und Klartext. Treffer gehen als `pipeline.Match` in den Trace, als JSON in `trace_stages.matches` in die DB und von dort in `/api/traces/{id}` und `/api/redactions` (`json_extract`). Ein neues JSON-Feld braucht deshalb keine Migration; alte Zeilen liefern eine leere Maske.

## Entscheidung
`Mask(rule, typ, value)` ist rein und deterministisch: gleiche Eingabe, gleiche Maske. `•` (U+2022) steht für verdeckte Zeichen. Klarzeichen sind Buchstaben und Ziffern aus dem Wert; ein Zeichen, das kein Buchstabe und keine Ziffer ist, wird nie gezeigt. Längen sind in Runen.

| Typ / Regel | Maske | Beispiel (erfunden) |
|---|---|---|
| E-Mail (`email`) | erstes Zeichen von Local-Part und erstem Domain-Label, TLD, keine Länge | `nora.beispiel@firma-beispiel.de` → `n•••@f•••.de` |
| API-Key mit bekanntem Präfix (`anthropic_key`, `openai_key`, `aws_access_key`, `github_token`, `slack_token`, `stripe_live`) | Präfix aus der Regel, ein Zeichen, Länge | `sk-ant-api03-a…` (108) → `sk-ant-api03-a••• (108)`; `AKIA` + 16 → `AKIA•••` bzw. `AKIAQ••• (20)` |
| JWT (`jwt`) | `eyJ`, ein Zeichen, Länge | `eyJhbGci…` (120) → `eyJh••• (120)` |
| IPv4 (`ipv4`) | erste Ziffer des ersten Oktetts, Struktur | `203.0.113.7` → `2•.•.•.•` |
| IPv6 (`ipv6`) | erstes Zeichen der ersten Gruppe, Struktur | `2001:db8::7` → `2•:•::•` |
| Passwort in Verbindungs-URL (`conn_string`) | nur Länge | `••• (12)` |
| Private Key (`pem_private`) | nur Länge | `••• (1704)` |
| Wortliste (`words`) | nur Länge | `••• (13)` |
| Heuristische Key-Regeln (`generic_assign`, `auth_header`, `bearer_token`, `client_secret`, `ps_secret_field`, `azure_cred_guid`, `cert_thumbprint`) | nur Länge | `••• (24)` |
| Eigene Regex | erstes Zeichen, Länge | `KD-123456` → `K••• (9)` |

Regeln, die für jede Maske gelten:
- Höchstens ein Klarzeichen je geheimem Segment. Öffentliche Strukturteile (Präfix aus der Regex, `eyJ`, TLD) zählen nicht als Segment.
- Alle Klarzeichen einschließlich der öffentlichen Teile machen höchstens ein Viertel der Runen des Werts aus, und die Maske ist höchstens 64 Bytes lang. Sonst greift die nächste, sparsamere Stufe derselben Regel (E-Mail: ohne TLD, dann nur Local-Part, dann `•••@•••`; Präfix-Keys: ohne Zusatzzeichen, dann nur Länge; IP: ohne Ziffer). Die letzte Stufe hat nie Klarzeichen.
- Wortliste, Passwörter, private Keys und heuristische Treffer (bei denen offen ist, ob ein Passwort gemeint ist) haben nie Klarzeichen.
- Unbekannte eingebaute Typen fallen auf „nur Länge“ zurück; nur Typen außerhalb der eingebauten (eigene Regex) zeigen ein Zeichen.

Abweichung von den Beispielen im Issue: `10.•.•.•` zeigte zwei Klarzeichen im ersten Oktett (und private Adressen redigiert tollgate ohnehin nicht); das verletzte die Regel „höchstens 1 je Segment“; die Regel geht vor, deshalb nur die erste Ziffer und feste Form `d•.•.•.•`, die keine Oktettlängen verrät. Die TLD zeigt das Issue ausdrücklich; sie gilt als öffentlicher Strukturteil.

Speicherung: Der Store nimmt nur eine leere Maske oder eine Maske mit mindestens einem `•`, gültigem UTF-8, ohne Steuerzeichen und mit höchstens 64 Bytes an. Ein versehentlich durchgereichter Klartext ohne `•` fällt so mit `ErrInvalid` auf.

## Alternativen
- Maske im Frontend aus dem Platzhalter: unmöglich, dort gibt es keinen Klartext.
- Hash statt Maske: eindeutig, aber für Menschen nicht wiedererkennbar; die Maske ist das Ziel des Issues.
- Eigenes Längenfeld: mehr Typ- und Schemaänderung ohne Nutzen; die Länge steht in der Maske.

## Risiken
- Die Maske verrät Länge und einzelne Zeichen. Das ist im Issue gewollt und durch Viertel- und Segmentgrenze gedeckelt.
- Cache: keine Wirkung, Upstream-Bodies und Diff ändern sich nicht (Test in `internal/redact`).

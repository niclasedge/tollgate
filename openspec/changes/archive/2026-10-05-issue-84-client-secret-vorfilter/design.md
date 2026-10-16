# Design

Verbindlich ist ADR-0005: Regex-Texte bleiben unverändert, ersetzt wird nur die
Secret-Gruppe. Beide Teile ändern nur, **welche Bytes** ersetzt werden bzw. **ob** eine Regex
läuft, nie das Verhalten der Regex selbst.

## 1. client_secret-Grenze

Neues optionales Feld `rule.trim func(text, start, end) (start, end)`, angewendet in `Find`
direkt nach dem Auslesen der Secret-Gruppe (vor Leer-, False-Positive-, Platzhalter- und
Allowlist-Prüfung). Nur `client_secret` nutzt es (`trimQuoted`):

- Steht direkt vor der Secret-Gruppe `"` oder `'` (das hat das `["']?` der Regex
  verbraucht), endet die Spanne am ersten gleichen Anführungszeichen, das nicht durch `\`
  maskiert ist (ein `\` überspringt das nächste Byte, damit `\\"` korrekt endet).
- Ist die Spanne danach leer (`client_secret="",…`), entfällt der Treffer wie jeder leere.
- Ohne öffnendes Anführungszeichen oder ohne schließendes bleibt die Spanne von `\S{8,}`.
  `client_secret=abcdefgh,` ersetzt also weiter das Komma mit: ohne Anführungszeichen ist
  nicht entscheidbar, ob das Komma zum Wert gehört, und lieber ein Zeichen zu viel als zu
  wenig.
- Die Mindestlänge 8 gilt für den Regex-Treffer, nicht für den gekürzten Wert:
  `client_secret="abcde",x` wird zu `client_secret="<redacted:api_key>",x`. Bisher wurde hier
  `abcde",x` ersetzt; einen kurzen Wert wieder stehen zu lassen, wäre ein Rückschritt.

Folge: `ClientSecret = 's3cretValue123abc'` liefert für `client_secret` und `ps_secret_field`
jetzt dieselbe Spanne; bei Gleichstand gewinnt die Regelreihenfolge (`ps_secret_field`). Die
Ausgabe ist `ClientSecret = '<redacted:api_key>'` (Korpusfall angepasst).

## 2. Vorfilter

Neue Felder je Regel: `need []string` (Literale, von denen jeder Treffer mindestens eines
enthält) und `fold bool` (`(?i)`-Regel, Literale klein geschrieben). `Find` erzeugt pro Aufruf
einen `prefilter`; `pass(rule)` prüft `strings.Contains` auf dem Originaltext bzw.
`bytes.Contains` auf dem gefalteten Text. Regeln ohne `need` (eigene Regex, Wortliste) laufen
immer.

| Regel | Literale |
|---|---|
| `anthropic_key` | `sk-ant-` |
| `openai_key` | `sk-` |
| `aws_access_key` | `AKIA` |
| `github_token` | `ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_` |
| `slack_token` | `xox` |
| `stripe_live` | `_live_` |
| `jwt` | `eyJ` |
| `pem_private` | `-----BEGIN ` |
| `email` | `@` |
| `conn_string` | `://` |
| `ipv4` | `.` |
| `ipv6` | `:` |
| `ps_secret_field` (`(?i)`) | `clientsecret`, `refreshtoken`, `accesstoken`, `apikey`, `apppassword`, `pscred`, `vaultpassword`, `safemode`, `servicepassword` |
| `azure_cred_guid` (`(?i)`) | `tenantid`, `clientid`, `appid`, `applicationid`, `objectid`, `subscriptionid`, `directoryid`, `principalid` |
| `cert_thumbprint` (`(?i)`) | `thumbprint`, `fingerprint` |
| `client_secret` (`(?i)`) | `clientsecret`, `client_secret` |
| `auth_header` (`(?i)`) | `authorization`, `x-api-key`, `x-auth-token`, `x-access-token` |
| `bearer_token` (`(?i)`) | `bearer`, `basic`, `digest` |
| `generic_assign` (`(?i)`) | `password`, `passwd`, `pwd`, `secret`, `api`, `access`, `auth`, `bearer` |

Jedes Literal ist ein Pflichtteil jeder Alternative der Regex (bei Alternativen eines je
Zweig), daher kann der Vorfilter keinen Treffer verlieren.

**Faltung:** Go-`(?i)` nutzt Unicode-Simple-Folding. Außer A–Z falten nur U+017F (ſ → s) und
U+212A (Kelvin-Zeichen → k) auf ASCII-Buchstaben; `TestFoldOrbitsToASCII` prüft das über alle
Runen. `foldASCII` bildet A–Z klein ab und ersetzt genau diese beiden Byte-Folgen. Da `C5`
und `E2` nie Folgebytes sind, decken sich die Byte-Folgen mit dem, was der Regex-Decoder als
Rune liest, auch in ungültigem UTF-8. Der gefaltete Text entsteht nur einmal je `Find` und
nur, wenn eine `(?i)`-Regel aktiv ist.

**Absicherung:** `TestPrefilterKeepsCorpusFindings` (ganzer Korpus, Standard- und
Korpus-Schalter, mit und ohne Vorfilter identische Treffer), `FuzzPrefilter` (beliebige
Eingaben, gleiche Bedingung), `TestPrefilterCaseFolding`, `TestPrefilterSkipsRules`
(prüft außerdem, dass jede eingebaute Regel Literale hat und `(?i)`-Literale klein sind).
Das Testfeld `Redactor.noPrefilter` schaltet den Vorfilter ab.

## Messung

`go test -run XXX -bench BenchmarkFind -benchtime 3s -count 3 ./internal/redact/`, Apple M4
Pro, Go 1.26.5. Eingabe: `benchBashOutput()`, 100 000 Byte deterministische Bash-Ausgabe
(go test, git log mit E-Mail-Adressen, npm, ls, JSON-Logs, Stacktraces, wenige Geheimnisse).
Vorher = Stand ohne #84 (gemessen vor der Umsetzung, gleicher Benchmark-Fall); `raw` = gleicher
Code mit abgeschaltetem Vorfilter.

| Fall | Vorher | Nachher (Vorfilter) | Nachher `raw` |
|---|---|---|---|
| `default` (Redact-Stufe) | 38,0 ms/op, 2,63 MB/s | 11,2 ms/op, 8,9 MB/s | 38,2 ms/op, 2,61 MB/s |
| `with-ip` (Korpus-Schalter) | 40,9 ms/op, 2,45 MB/s | 14,4 ms/op, 7,0 MB/s | 41,7 ms/op, 2,40 MB/s |
| `all-keywords` (jedes Literal im Text) | – | 38,7 ms/op, 2,59 MB/s | 38,5 ms/op, 2,60 MB/s |

Faktor rund 3,4 im typischen Fall. Im schlechtesten Fall (alle Literale vorhanden, nichts
übersprungen) kostet der Vorfilter etwa 0,3 ms (unter 1 %) und rund 100 KB zusätzliche
Allokation für den gefalteten Text. Die verbleibenden 11 ms entfallen auf die Regeln, deren
Literal in der Ausgabe tatsächlich vorkommt (`email` wegen der git-log-Adressen, `jwt`,
`auth_header`, `bearer_token`, `generic_assign`).

## Alternativen

- **Regex-Text von `client_secret` ändern** (`[^\s"']{8,}`): ändert, was trifft, und bricht die
  Regel „Regex-Texte unverändert“ aus ADR-0005.
- **Anführungszeichen und Komma am Ende abschneiden:** scheitert an kompaktem JSON
  (`"geheim","id":1}` endet auf `}`).
- **Literale automatisch aus der Regex ableiten** (`regexp/syntax`): allgemeiner, aber
  deutlich mehr Code für 19 feste Regeln; die Tabelle ist durch die Tests abgesichert.
- **Fenster-Suche um Literale:** würde auch die verbleibenden Regeln beschleunigen, ist aber
  wegen zeilenübergreifender Muster (`\s*`, `[\s\S]`) nicht trivial korrekt. Offener Punkt.

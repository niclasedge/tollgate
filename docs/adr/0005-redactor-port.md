# ADR-0005: Redactor-Port nach Go

- **Status:** Angenommen (Freigabe Niclas, 2026-10-04, PR #26)
- **Datum:** 2026-10-04
- **Issue:** #1

## Kontext

Der Rust-Redactor (`ref/hook-redactor`, v0.8.1, MIT, „Copyright (c) 2026 Niclas Edge“)
ist eine Claude-Code-Hook-CLI mit etwa 18.000 Zeilen. Der für tollgate relevante Kern umfasst
rund 1.100 Zeilen:

- `src/patterns.rs`: `Rule{name, regex, replacement}`, `defaults()` mit 26 statischen Regeln,
  `dynamic_username_rule()`, `load_custom()`, `from_keywords()`
- `src/redactor.rs`: `redact_pattern()`, False-Positive-Filter (`is_false_positive`,
  `WHITELIST_SUBSTRINGS`, private IPv4/IPv6, Versions-Kontext), CSV-, JSON- und env-Modi
- `src/config.rs`: `.redactor.toml`
- `src/ignorefile.rs`: Keyword-Liste `~/.redactorignore`

Befunde:

- **Regex-Engine:** reine `regex`-Crate, keine Lookarounds, keine Backreferences
  (`knowledge/decisions.md`: „Rust regex has no lookahead“). Alle Muster kompilieren in Go
  `regexp` (RE2), beide Engines wählen leftmost-first.
- **Unicode:** `\b`, `\d` und `\s` sind in Rust Unicode-bewusst, in Go nur ASCII.
- **Sequenzielle Ersetzung:** `redact_pattern` wendet jede Regel per `replace_all` auf die
  Ausgabe der vorigen an. Folgen:
  - `api_key=sk-ant-…` wird zuerst `<redacted:anthropic_key>`, dann überschreibt
    `generic_assign` zu `api_key=<redacted>`. Das Label geht verloren.
  - `client_secret` und `auth_header` matchen ihre eigenen Platzhalter erneut.
- **Template-Bug:** `windows_home_dir` nutzt `"$1Users$2…"`. Beide Engines lesen `$1Users`
  als Gruppennamen, der Laufwerksbuchstabe verschwindet.
- **Keine Nummerierung, keine Rückübersetzung.** Platzhalter sind feste Labels
  (`<redacted:email>`).
- **Keine Inhalts-Allowlist** außer dem fest eingebauten `WHITELIST_SUBSTRINGS`.
- **Keywords** sind nur Teilstring-Treffer ohne Groß-/Kleinschreibung, ohne Option für ganze
  Wörter.
- **Tests:** etwa 545 `#[test]`, davon rund 200 zur Redaktion, alle als Inline-Assertions
  (`contains` / `!contains`). Es gibt keinen Korpus und keine Golden-Files. Am ergiebigsten:
  - `src/patterns.rs` (61)
  - `src/redactor.rs` (21)
  - `tests/red_team.rs` (67)
  - `tests/eu_pii_patterns.rs` (23)
  - `tests/integration.rs` (25)

## Entscheidung

### 1:1 portiert

- **Regex-Texte** der unten zugeordneten Regeln, unverändert bis auf `${n}` statt `$n`.
- **False-Positive-Filter:**
  - private und lokale IPv4 (10/8, 127/8, 0.0.0.0, 172.16/12, 192.168/16)
  - IPv6 (`::1`, fe80::/10, fc00::/7, Mindestform)
  - Versions-Kontext (Anführungszeichen-Paar oder Token wie `version`/`v`/`build` im
    Fenster von ±16 Zeichen)
  - E-Mail-Ausnahmen (example.com, `.test`, localhost, `noreply@`, `no-reply@`)
  - Test-IBAN
- **Keyword-Escaping** (`regexp.QuoteMeta`)
- **Testfälle:** Positive und negative Fälle aus den genannten Testdateien kommen als
  txtar-Korpus nach `internal/redact/testdata/`. Format pro Fall:
  ```
  -- input --
  …
  -- must_contain --
  …
  -- must_not_contain --
  …
  ```
- **Grenzfälle** aus `tests/red_team.rs` §7 werden als Tests übernommen, die das aktuelle
  Verhalten festschreiben. Das sind URL-kodierte Secrets und Secrets über Zeilengrenzen.
  §9 (False-Positive-Fläche) wird ebenfalls übernommen.

### Zuordnung zu den eingebauten Schaltern

| Schalter (`builtin.*`) | Standard | Rust-Regeln |
|---|---|---|
| `email` | an | `email` |
| `api_keys` | an | `anthropic_key`, `openai_key`, `aws_access_key`, `github_token`, `slack_token`, `stripe_live`, `bearer_token`, `auth_header`, `generic_assign`, `ps_secret_field`, `client_secret`, `azure_cred_guid`, `cert_thumbprint` |
| `jwt` | an | `jwt` |
| `connection_urls` | an | `conn_string` (neu gefasst, siehe unten) |
| `ip_addresses` | aus | `ipv4`, `ipv6` |
| `private_keys` | an | `pem_private` |

**Nicht portiert:**
- `crypto_container_path`, `windows_home_dir`, `unix_home_dir`, `current_username`,
  `internal_hostname`, `iban_de`, `birthdate_de`, `svn_de`
- die CSV-, JSON- und env-Strukturmodi

Begründung: Diese Regeln sind entweder hook-spezifisch (Pfade, Username aus der Umgebung)
oder haben eine hohe False-Positive-Rate (`birthdate_de` trifft jedes deutsche Datum). Bei
Bedarf werden sie später als zusätzliche Schalter nachgezogen, standardmäßig aus, jeweils
als eigenes Issue.

Ebenfalls nicht portiert: Hook-, Path-Guard-, Such-, Markdown- und Klassifikator-Code.

### Neu entworfen

1. **Erst alle Treffer sammeln, dann auflösen:**
   - Alle aktiven Detektoren laufen auf dem **Originaltext** und liefern Treffer
     `{start, end, typ, priorität}`.
   - Überlappungen werden aufgelöst nach: höhere Priorität, dann längere Spanne, dann
     früherer Start.
   - Priorität: Wortliste und eigene Regex vor spezifischen eingebauten Regeln
     (Anbieter-Keys, JWT, PEM, E-Mail, URL) vor generischen (`generic_assign`,
     `auth_header`, `bearer_token`).
   - Danach wird in einem Durchgang ersetzt.
   - Das behebt den Labelverlust und das erneute Matchen eigener Platzhalter. Text in
     Platzhalterform wird außerdem nie als Treffer gewertet.
2. **Secret-Gruppe statt Replacement-Template:**
   - Jede Regel benennt die Capture-Gruppe, die das Geheimnis enthält (Standard: der ganze
     Treffer).
   - Präfixe wie `password=`, `TenantId=` oder `Authorization: Bearer ` bleiben stehen.
   - Templates wie `$1Users$2` entfallen, damit auch der Template-Bug.
3. **`connection_urls` nur mit Passwort:**
   - Das neue Muster verlangt Benutzerinfo mit Passwort:
     `(?i)\b(postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis|rediss|amqps?)://[^\s:/@]+:([^\s@/]+)@`
   - Ersetzt wird nur Gruppe 2, z. B. `postgres://app:<redacted:connection_password_1>@db:5432/x`.
   - URLs ohne Passwort bleiben lesbar.
4. **Konfigurierbarkeit** (Schema siehe ADR-0008):
   - eingebaute Regeln über die Schalter oben
   - eigene Regex: `{name, pattern, placeholder, examples[]}`. Beim Laden muss jedes Beispiel
     mindestens einmal treffen, sonst wird die Konfiguration abgelehnt. `placeholder` muss
     der Platzhalter-Grammatik entsprechen (ADR-0006).
   - Wortliste: `case_sensitive` (Standard `false`) und `whole_word` (Standard `true`). Wörter
     werden escaped. `whole_word` prüft die Grenze manuell Unicode-bewusst
     (`unicode.IsLetter`/`IsDigit` links und rechts), weil Go-`\b` nur ASCII kennt. Mehrwort-
     Einträge wie „Projekt Falke“ matchen mit flexiblem Whitespace (`\s+`).
   - Allowlist: exakte Werte, Vergleich nach der Erkennung auf dem Trefferwert. Sie ersetzt
     und verallgemeinert `WHITELIST_SUBSTRINGS`. Die eingebauten E-Mail-Ausnahmen bleiben
     zusätzlich aktiv.
5. **Platzhalter mit Nummerierung** (ADR-0006): `<redacted:TYP_N>`. Die Typen der eingebauten
   Regeln sind `email`, `api_key`, `jwt`, `connection_password`, `ip`, `private_key`. Bei
   `api_keys` bleibt der konkrete Anbieter nur im Trace sichtbar (Regelname), nicht im
   Platzhalter.

### Paritätstest

- Test `internal/redact/parity_test.go`, gesteuert über `TOLLGATE_REDACTOR_REPO` (Standard
  `ref/hook-redactor`). Er wird übersprungen, wenn dort kein Repo oder kein Binary
  `target/release/redactor` liegt.
- **Aufruf** pro Korpus-Eintrag:
  `redactor -t pattern --no-log -` in einem leeren temporären Verzeichnis, mit
  `REDACTOR_CONFIG_DIR` und `REDACTOR_IGNOREFILE_PATH` auf nicht existierende Pfade. So werden
  weder `.redactor.toml` noch Keywords geladen, und die Home-Dir-Regeln sind aus.
- **Vergleich:**
  - Rust ohne die nicht portierten Regeln gegen Go mit den Standardschaltern plus
    `ip_addresses: true`.
  - Normalisiert werden die Platzhalter: `<redacted:TYP_N>` wird zu `<redacted>`, Rust-Labels
    ebenso. Das abschließende `\n` der Rust-Ausgabe wird entfernt.
- **Bewusste Abweichungen.** Sie stehen als benannte Liste im Test, jeweils mit Verweis auf
  diesen Abschnitt:
  1. Überlappungen: Go behält spezifische Labels, Rust überschreibt mit `generic_assign`.
  2. `connection_urls`: Go ersetzt nur das Passwort und lässt URLs ohne Passwort stehen.
  3. Präfixe bleiben stehen (Secret-Gruppe), wo Rust Teile des Präfixes mit ersetzt.
  4. Unicode: Nicht-ASCII-Whitespace, -Ziffern und -Wortgrenzen. Go folgt RE2-ASCII-Semantik,
     die Fälle sind im Korpus explizit enthalten.

Jede andere Abweichung lässt den Test fehlschlagen. Sie ist dann entweder ein Fehler im Port
oder muss hier nachgetragen und begründet werden.

### Nachtrag zur Umsetzung (#12, 2026-10-04)

Präzisierungen, die sich beim Port ergeben haben. Sie ändern keine Entscheidung oben.

- **Prioritäten im Detail:** `conn_string` liegt über den übrigen spezifischen Regeln, weil
  `user:pass@host.tld` sonst als E-Mail gelesen würde und das längere E-Mail-Match den Host
  verdeckte. Regeln, die nur über einen Schlüsselnamen erkennen (`ps_secret_field`,
  `azure_cred_guid`, `cert_thumbprint`, `client_secret`), zählen wie `generic_assign`,
  `auth_header` und `bearer_token` als generisch. Bei gleicher Priorität, Länge und
  Startposition entscheidet die Reihenfolge der Regeltabelle (Anbieter-Keys zuerst, damit
  `sk-ant-…` als `anthropic_key` und nicht als `openai_key` im Trace steht).
- **Secret-Gruppe:** Bei `auth_header`, `azure_cred_guid`, `cert_thumbprint` und
  `client_secret` kommt eine Klammer um den Wertteil hinzu. Was die Regex trifft, ändert sich
  dadurch nicht. Bei `client_secret` schließt `\S{8,}` ein schließendes Anführungszeichen mit
  ein; es wird mit ersetzt (Korpusfall `patterns/client_secret_snake_case`).
- **E-Mail-Ausnahmen:** portiert ist die Rust-Liste `example.com`, `example.test`,
  `example.org`, `localhost`, `noreply@`, `no-reply@` (Teilstring) und für alle Regeln der
  exakte Vergleich mit diesen Werten. Der Test-IBAN-Filter entfällt mit `iban_de`.
- **Korpus:** ein Fall je Datei unter `internal/redact/testdata/corpus/<herkunft>/`. Neben
  `input`, `must_contain` und `must_not_contain` gibt es optional `want` (exakte Go-Ausgabe)
  und `deviation` (Name einer bewussten Abweichung). `{{}}` wird beim Laden entfernt und
  trennt schlüsselartige Testwerte, damit Secret-Scanner nicht anschlagen.
- **Neutralisierung im Paritätstest:** Der Aufruf lautet
  `redactor -t pattern --no-log --no-defaults --no-config --patterns <json> -`. Die
  JSON-Datei enthält genau die portierten Regeln, gelesen aus `defaults()` in
  `src/patterns.rs` des Referenz-Repos (Regex und Template unverändert, Rust-Reihenfolge).
  So läuft Rust ohne die nicht portierten Regeln, ohne dass der Korpus sie umgehen muss. Die
  False-Positive-Filter greifen weiter, weil Rust sie über den Regelnamen auswählt.
  Leeres Arbeitsverzeichnis und nicht existierende `REDACTOR_CONFIG_DIR` und
  `REDACTOR_IGNOREFILE_PATH` bleiben zusätzlich gesetzt.
- **Abweichungen:** Die vier Punkte oben decken alle 23 abweichenden Korpusfälle ab
  (Namen im Test: `overlap`, `connection-url`, `prefix`, `unicode`). Zu Punkt 3 zählt auch,
  dass Rust-Templates Trenner umschreiben (`TenantId: x` wird `TenantId=…`,
  `Authorization=x` wird `Authorization= …`). Ein Fall mit markierter Abweichung, die nicht
  mehr auftritt, lässt den Test ebenfalls fehlschlagen.

### Nachtrag: Rest verdrängter generischer Treffer (#83, 2026-10-05)

Bisher fiel ein generischer Treffer ganz weg, wenn ein spezifischer Treffer in seiner
Secret-Spanne lag. Bei `Authorization: Bearer <jwt> weiterer-text` blieb `weiterer-text` im
Klartext, weil `auth_header` den Wert bis zum Zeilenende greift.

- **Restspannen:** Wird ein Treffer mit generischer Priorität durch Überlappung verdrängt,
  wird jeder nicht abgedeckte Lauf seiner Secret-Spanne ein eigener Treffer mit Regel und Typ
  des verdrängten Treffers. Die Auflösung bleibt ein Durchgang in der bestehenden
  Reihenfolge; der Rest wird sofort übernommen und deckt seine Bytes ab.
- **Was bleibt stehen:** Ein Rest wird an den Rändern um Whitespace gekürzt. Er entfällt,
  wenn er keinen Buchstaben und keine Ziffer enthält (schließende Anführungszeichen), ganz im
  Nicht-Secret-Teil eines anderen Kandidaten liegt (Schlüsselname `password=` von
  `generic_assign`, Schema von `bearer_token`), ein Authentifizierungsschema `Bearer`,
  `Basic` oder `Digest` ist (auch wenn das Token für `bearer_token` zu kurz ist) oder in der
  Allowlist steht.
- **Nur generische Treffer:** Bei spezifischen Regeln ist der Rest bewusst kein Geheimnis,
  z. B. `@db.prod.io`, wenn `conn_string` die E-Mail-Lesart von `app:pass@db.prod.io`
  verdrängt.
- **Parität:** Rust ersetzt in diesem Fall den ganzen Header-Wert
  (`Authorization: <redacted:auth_header>`), Go ergibt
  `Authorization: Bearer <redacted:jwt> <redacted:api_key>`. Das fällt unter die bewusste
  Abweichung 1 (`overlap`): Go behält das spezifische Label und ersetzt den Rest gesondert,
  statt den spezifischen Treffer zu überschreiben. Korpusfall
  `tollgate/residual_of_displaced_generic`. Die bestehenden `overlap`-Fälle ändern ihre
  Go-Ausgabe nicht; der Paritätstest vergleicht jetzt 134 Fälle mit 24 bewussten
  Abweichungen.

### Nachtrag: client_secret-Grenze und Vorfilter (#84, 2026-10-05)

- **client_secret:** Der Regex-Text bleibt, aber die Secret-Spanne wird gekürzt: Folgt der
  Wert auf ein öffnendes `"` oder `'`, endet er am ersten nicht mit `\` maskierten gleichen
  Anführungszeichen. Damit gilt die Aussage im Nachtrag zu #12 („schließt ein schließendes
  Anführungszeichen mit ein; es wird mit ersetzt“) nicht mehr. Unquotierte Werte behalten die
  Spanne von `\S{8,}`. Die Abweichung zu Rust bleibt `prefix` (Rust schreibt den Trenner um
  und ersetzt bis zum nächsten Whitespace). Korpusfälle `patterns/client_secret_snake_case`,
  `patterns/ps_secret_client_secret` (jetzt gleiche Spanne, `ps_secret_field` gewinnt über
  die Regelreihenfolge) und neu `tollgate/client_secret_json`.
- **Vorfilter:** Jede eingebaute Regel trägt Literale, von denen jeder ihrer Treffer
  mindestens eines enthält; fehlen alle, läuft die Regex nicht. Für `(?i)`-Regeln wird der Text
  einmal ASCII-gefaltet, zusätzlich U+017F → `s` und U+212A → `k`, die einzigen Nicht-ASCII-
  Runen, die Go-`(?i)` auf ASCII-Buchstaben faltet. Das ändert keine Ausgabe; Tests verlangen
  über den ganzen Korpus und per Fuzzing identische Treffer mit und ohne Vorfilter. Messung
  (100 KB Bash-Ausgabe, Standard-Schalter): 38 ms → 11 ms.
- Der Paritätstest vergleicht danach 135 Korpusfälle mit 25 bewussten Abweichungen.

## Alternativen

- **Rust-Redactor per FFI oder als Subprozess:** verworfen. cgo verletzt das statische
  Binary, ein Subprozess pro Request ist zu langsam, und es gäbe weder Streaming noch
  Konfigurierbarkeit.
- **Sequenzielle Ersetzung beibehalten** (exakte Parität): verworfen. Labelverlust und erneutes
  Matchen eigener Platzhalter widersprechen den nummerierten, umkehrbaren Platzhaltern aus
  ADR-0006.
- **Ganze Verbindungs-URL ersetzen** wie in Rust: verworfen. Der Auftrag spricht von
  „Verbindungs-URLs mit Passwort“, und die Lesbarkeit von Host und Datenbank hilft dem Modell.

## Konsequenzen

- **#12** setzt Port, Korpus und Paritätstest um.
- **#13** setzt die Platzhalter um, **#15** die Konfigurierbarkeit.
- Der Korpus ist die gemeinsame Basis für den Live-Test im Filter-Bereich (#16) und den
  Dry-Run beim YAML-Import.

## Quellen

hook-redactor v0.8.1:
- `src/patterns.rs` (`defaults()`, Regeln in Zeilen 32–215)
- `src/redactor.rs` (`redact_pattern`, `is_false_positive`, `WHITELIST_SUBSTRINGS`)
- `src/config.rs`, `src/ignorefile.rs`, `src/main.rs` (`run_filter`)
- `tests/red_team.rs`, `tests/eu_pii_patterns.rs`, `tests/integration.rs`
- `knowledge/decisions.md`, `knowledge/lessons-learned.md`, `LICENSE`

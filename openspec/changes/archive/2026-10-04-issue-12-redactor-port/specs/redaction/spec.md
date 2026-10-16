## ADDED Requirements

### Requirement: Eingebaute Detektoren nach ADR-0005

`internal/redact` SHALL die 19 in ADR-0005 zugeordneten Rust-Regeln mit unveränderten Regex-Texten, den False-Positive-Filtern (private IPv4 und IPv6, Versions-Kontext, E-Mail-Ausnahmen, exakte Whitelist) und den sechs Schaltern `email`, `api_keys`, `jwt`, `connection_urls`, `ip_addresses`, `private_keys` bereitstellen. `ip_addresses` MUST standardmäßig aus sein, alle anderen an. `connection_urls` MUST nur das Passwort in der Benutzerinfo einer Verbindungs-URL treffen. Die nicht portierten Regeln aus ADR-0005 MUST NOT enthalten sein.

#### Scenario: Schalter aus
- **WHEN** `ip_addresses` aus ist und der Text `DNS 8.8.8.8` enthält
- **THEN** bleibt die Adresse stehen; mit `ip_addresses: true` wird sie ersetzt.

#### Scenario: Verbindungs-URL
- **WHEN** der Text `postgres://app:s3cretpw@db.prod.io:5432/x` enthält
- **THEN** lautet die Ausgabe `postgres://app:<redacted:connection_password>@db.prod.io:5432/x`, und eine URL ohne Passwort bleibt unverändert.

#### Scenario: False Positive
- **WHEN** der Text `Version 1.2.3.4 released` oder `noreply@example.org` enthält
- **THEN** wird nichts ersetzt.

### Requirement: Erst sammeln, dann auflösen

Alle aktiven Detektoren SHALL auf dem Originaltext laufen. Überlappende Treffer MUST nach Priorität, dann Spannenlänge, dann Startposition, dann Regelreihenfolge aufgelöst werden, und die Ersetzung MUST in einem Durchgang erfolgen. Ersetzt wird nur die Secret-Gruppe einer Regel; Präfixe wie `password=` oder `Authorization: Bearer ` bleiben stehen. Text in Platzhalterform nach ADR-0006 MUST NOT als Treffer gelten.

#### Scenario: Spezifisch vor generisch
- **WHEN** der Text `api_key=sk-ant-…` enthält
- **THEN** stammt der Treffer von `anthropic_key`, und die Ausgabe ist `api_key=<redacted:api_key>`.

#### Scenario: Vorhandener Platzhalter
- **WHEN** der Text `password=<redacted:api_key_1>` enthält
- **THEN** bleibt er unverändert.

### Requirement: Determinismus und Robustheit

Gleiche Eingabe und gleiche Optionen SHALL Byte für Byte die gleiche Ausgabe und die gleichen Treffer liefern. Beliebige Eingaben, auch ungültiges UTF-8, MUST ohne Panic verarbeitet werden, und der Klartext eines übernommenen Treffers MUST NOT in der Ausgabe stehen, sofern er nicht an anderer, nicht erkannter Stelle der Eingabe vorkommt.

#### Scenario: Wiederholter Aufruf
- **WHEN** derselbe Text zweimal redigiert wird
- **THEN** sind Ausgabe und Treffer identisch.

### Requirement: Korpus und Paritätstest

Die relevanten Testfälle aus `src/patterns.rs`, `src/redactor.rs`, `tests/red_team.rs` (einschließlich §7 und §9), `tests/eu_pii_patterns.rs` und `tests/integration.rs` SHALL als txtar-Korpus unter `internal/redact/testdata/` vorliegen und gegen den Go-Port laufen. `parity_test.go` SHALL denselben Korpus durch das Rust-Binary aus `TOLLGATE_REDACTOR_REPO` (Standard `ref/hook-redactor`) schicken und normalisiert vergleichen; ohne Repo oder Binary MUST der Test übersprungen werden. Jede Abweichung, die nicht als bewusste Abweichung aus ADR-0005 benannt und im Fall markiert ist, MUST den Test fehlschlagen lassen, ebenso eine markierte Abweichung, die nicht mehr auftritt.

#### Scenario: Rust-Repo vorhanden
- **WHEN** `ref/hook-redactor/target/release/redactor` existiert
- **THEN** läuft jeder Korpusfall durch beide Implementierungen, und nur markierte Fälle unterscheiden sich.

#### Scenario: Rust-Repo fehlt
- **WHEN** `TOLLGATE_REDACTOR_REPO` auf ein Verzeichnis ohne Binary zeigt
- **THEN** wird der Paritätstest übersprungen.

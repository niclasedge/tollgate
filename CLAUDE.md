# Auftrag: tollgate – Gateway mit Pipeline, Backend und Frontend (Go)

Diese Datei ist die führende Regeldatei für Claude Code und Codex; `CLAUDE.md`
importiert sie nur. Ergänzend verbindlich: [`docs/standards.md`](docs/standards.md)
(Produkt- und Prozessstandards) und [`docs/ops-lab.md`](docs/ops-lab.md) (agentischer
Ablauf). Lokale Einrichtung der Agent-Werkzeuge: [`docs/AGENT_SETUP.md`](docs/AGENT_SETUP.md).

> **Repo-Status:** vorerst **privat** unter `niclasedge/tollgate`. Wird später
> **public** gestellt. Deshalb ab dem ersten Commit: keine Secrets, keine internen
> Hostnamen, keine Kundennamen und keine ungeklärten Lizenzen ins Repo.

## Ziel
Baue tollgate: ein nachvollziehbares, testbares Gateway für alle Claude-API-Aufrufe aus
Claude Code. Jede Anfrage läuft durch eine feste Pipeline aus Redaction, Kompression,
Messung und Weiterleitung. Jede Stufe ist im Frontend Schritt für Schritt nachvollziehbar.

Grundprinzip: **Alles passiert im Proxy.** Auf den Clients gibt es keine Hooks, kein rtk,
keine CLAUDE.md-Injektion und keine Wrapper. Ein Client setzt nur die URL und einen
Client-Key, tollgate macht den Rest.

## Referenzen
Redactor und Headroom sind **reine lokale Referenzen** für die Entwicklung. Sie liegen
als Symlinks unter `ref/` im Repo, `ref/` steht in `.gitignore` und wird nie committet:
keine Kopie, kein Submodul, kein Vendoring.

- Redactor (Rust, eigenes Repo): `ref/hook-redactor`
  (→ `$HOME/src/hook-redactor`)
  → wird **nach Go portiert** als Paket `internal/redact`. Das Rust-Repo dient nur als
  Referenz für Logik und Testfälle.
- Headroom (Python): `ref/headroom` (→ `$HOME/src/headroom`)
  → nur als Referenz. Relevante Kompressionsstrategien in Go portieren.
- CLIProxyAPI (Go): https://github.com/router-for-me/CLIProxyAPI
  → **keine Kopie im Repo.** Der Container-Build holt CPA per `go install` gepinnt auf
  einen Commit (`ARG CPA_VERSION` in `deploy/cliproxyapi/Dockerfile`, die einzige Stelle).
  Commit, Lizenz und Update-Ablauf stehen in `deploy/cliproxyapi/UPSTREAM.md` (ADR-0014).
  Lokale Referenz zum Lesen: `ref/cliproxyapi` (Klon auf den Pin, gitignored).
- Prüfe vor jedem Kopieren oder Portieren die LICENSE-Dateien. Behalte Lizenz- und
  Copyright-Hinweise bei und dokumentiere die Herkunft in `third_party/NOTICE.md`.
- Jede Änderung an kopiertem Code wird in `third_party/<name>/PATCHES.md` begründet.
  CLIProxyAPI bleibt ungepatcht; ein Patch bräuchte ein neues ADR.

## Arbeitsweise
1. Lies zuerst alle drei Quellen und schreibe `docs/ARCHITECTURE.md` sowie ADRs unter
   `docs/adr/`. Mindestens zu diesen Fragen:
   - Läuft CLIProxyAPI als eigener Prozess, oder lässt er sich ohne Fork als Bibliothek
     einbinden? Bevorzugt wird die Lösung, bei der Upstream-Updates einfach bleiben.
   - Welche Teile des Rust-Redactors werden 1:1 portiert, welche neu entworfen?
   - Was wird aus Headroom portiert?
   - Welche Bibliotheken werden verwendet (Router, SQLite, YAML, Verschlüsselung)?
   - Wie sieht das Konfigurationsschema aus?
2. **Warte auf meine Freigabe**, bevor du Code schreibst.
3. Danach baust du in Phasen. Jede Phase endet mit grünen Tests und einem kurzen
   Bericht: was gebaut wurde, wie es getestet ist, was offen ist.

## Tracking: GitHub Issues + OpenSpec
- Alle geplanten Features, Aufgaben und Bugs sind **GitHub Issues** in
  `niclasedge/tollgate` (nicht beads, nicht backlog.md).
- Abhängigkeiten sind als GitHub-Dependencies „blocked by“ gesetzt und zusätzlich im
  Issue-Text als `Depends on #N` notiert. Ein Issue wird erst begonnen, wenn alle
  Abhängigkeiten geschlossen sind.
- Phasen sind Milestones (`Phase 0` bis `Phase 4`).
- **Ein Issue = ein OpenSpec-Change.** Ablauf:
  1. Change anlegen: `/opsx:propose`, Change-ID `issue-<N>-<slug>`, Issue im
     `proposal.md` verlinken.
  2. Proposal, Specs und Tasks ausarbeiten, `openspec validate` muss grün sein.
  3. Umsetzen (`/opsx:apply`), Tests grün.
  4. PR mit `Closes #<N>`, nach dem Merge Change archivieren (`/opsx:archive`).
- Neue Erkenntnisse während der Arbeit werden als neues Issue erfasst, nicht im
  laufenden Change mitgezogen.

## Architektur
Datenfluss:
Claude Code → tollgate (Go) → CLIProxyAPI (localhost) → Anthropic
Die Antwort läuft rückwärts durch tollgate.

Zielstruktur (Vorschlag, in ADR bestätigen oder begründet ändern):
```
tollgate/
  AGENTS.md                     Projektregeln für künftige Sessions (führend)
  CLAUDE.md                     importiert nur AGENTS.md
  cmd/tollgate/                 main, ein einziges Binary
  internal/
    pipeline/                   Stage-Interface, Kontext, Trace-IDs
    redact/                     Go-Port des Redactors
    compress/                   Regeln pro Befehl + Headroom-Port
    meter/                      Token- und Kostenmessung
    config/                     Schema, YAML-Import/-Export, Versionen, Verschlüsselung
    store/                      SQLite (Traces, Messwerte, Konfigurationsversionen)
    upstream/                   Client zu CLIProxyAPI
    server/                     HTTP-Proxy, Frontend-API, SSE, eingebettetes Frontend
  web/                          Frontend (Svelte)
  config/schema/                JSON-Schema für den YAML-Import
  deploy/cliproxyapi/           Dockerfile mit CPA-Pin, UPSTREAM.md, Konfigurationsvorlage
  third_party/NOTICE.md
  testdata/fixtures/            aufgezeichnete, bereinigte Anfragen
  docs/ARCHITECTURE.md, docs/adr/, docs/CLIENT_SETUP.md
  docker-compose.yml
```

Technische Vorgaben (im ADR bestätigen):
- SQLite ohne cgo (z. B. `modernc.org/sqlite`), damit tollgate ein statisches Binary bleibt.
- Frontend per `embed` ins Binary.
- Logging mit `log/slog`, Tracing optional mit OpenTelemetry.

Pipeline-Stufen, jede als eigene Implementierung eines gemeinsamen `Stage`-Interfaces:
0. **Eingang:** Client-Key prüfen, `/v1/messages` parsen, Trace-ID vergeben.
1. **Redact:** Geheimnisse durch nummerierte, deterministische Platzhalter ersetzen,
   z. B. `<redacted:email_1>`. Die Zuordnung liegt nur im RAM, mit TTL pro Sitzung.
2. **Compress:** Nur `tool_result` von Bash-Aufrufen, mit einer Regel pro Befehl
   (`cargo test`, `go test`, `git log`, `npm install` usw.). Den Befehl liest du aus dem
   zugehörigen `tool_use` im Verlauf. Das ersetzt rtk vollständig.
3. **Meter:** Vorab eine Schätzung loggen, nach der Antwort das `usage`-Feld
   (`input`, `output`, `cache_read`, `cache_creation`) speichern.
4. **Forward:** Weiterleiten an CLIProxyAPI auf `localhost`.
5. **Rückweg:** Platzhalter beim Streamen zurückübersetzen, auch in `input_json_delta`.
   Puffern, bis ein Platzhalter vollständig ist.

Alle anderen Endpunkte, die Claude Code aufruft (z. B. `count_tokens`, Modelllisten),
werden unverändert durchgereicht. Header wie `anthropic-version` und `anthropic-beta`
bleiben erhalten.

## Client-Einrichtung
- Ein Client braucht nur zwei Umgebungsvariablen: `ANTHROPIC_BASE_URL` (tollgate im
  Tailnet) und `ANTHROPIC_AUTH_TOKEN` (sein Client-Key von tollgate).
- Prüfe in der aktuellen Claude-Code-Dokumentation, welche Variablen genau gelten, und
  dokumentiere das in `docs/CLIENT_SETUP.md`.
- Das Frontend erzeugt pro Client einen Key und zeigt ein kopierfertiges Setup-Snippet.
- Keys lassen sich im Frontend sperren und rotieren.

## Harte Regeln
- Niemals Read-, Edit- oder Datei-Inhalte komprimieren, nur Bash-Ausgaben.
- Alle Transformationen sind deterministisch: gleiche Eingabe, Byte für Byte gleiche
  Ausgabe. Keine Zeitstempel, kein Zufall, keine Map-Iterationsreihenfolge. Sonst
  bricht das Prompt-Caching.
- Thinking-Blöcke und ihre Signaturen werden nie verändert. Nur User-Text und
  `tool_result` anfassen.
- Unbekannte Content-Typen und Felder werden unverändert durchgereicht. JSON wird so
  verarbeitet, dass nicht modellierte Felder nicht verloren gehen.
- In Logs, DB und Frontend landen nur redigierte Daten, nie Klartext-Geheimnisse. Einzige
  Ausnahme: „Bearbeiten“ der Kunden-, Vornamen- und Nachnamenliste nach ADR-0021 (Klartext
  nur auf ausdrücklichen Klick im Frontend).
- tollgate bindet nur an die Tailnet-IP, CLIProxyAPI nur an `localhost`. Im Container gilt
  das für den von Docker veröffentlichten Socket auf der Tailnet-IP des Hosts (ADR-0013).
- Im auth-dir von CLIProxyAPI liegt immer nur die Token-Datei des aktiven Accounts.
  Das Frontend zeigt an, welcher Account aktiv ist.

## Redactor-Port nach Go
- Übernimm alle Testfälle aus dem Rust-Repo als Go-Testdaten unter
  `internal/redact/testdata/`.
- Parität: Ein Differenztest lässt Rust- und Go-Version über denselben Korpus laufen.
  Abweichungen sind entweder Fehler oder werden im ADR als bewusste Änderung begründet.
  Der Test findet das Rust-Repo über `TOLLGATE_REDACTOR_REPO` (Default
  `ref/hook-redactor`) und wird übersprungen, wenn es dort nicht existiert.
- Danach erweiterst du den Port um die Konfigurierbarkeit (nächster Abschnitt).

## Redactor-Konfiguration
Die gesamte Konfiguration ist über das Frontend pflegbar und als YAML im- und
exportierbar. Beide Wege führen zum selben Ergebnis.

- Die Konfiguration liegt versioniert in der DB. Jede Änderung erzeugt eine neue
  Version mit Zeitstempel und Diff, ein Rollback ist mit einem Klick möglich.
- YAML-Import: zuerst gegen `config/schema/` validieren und einen Dry-Run auf
  `testdata/fixtures/` laufen lassen. Erst nach Bestätigung aktivieren.
- Hot Reload ohne Neustart.
- Filtertypen:
  - Eingebaute Filter, einzeln schaltbar: E-Mail, API-Keys und Tokens, JWT,
    Verbindungs-URLs mit Passwort, IP-Adressen, private Keys.
  - Eigene Regex-Filter mit Name, Platzhalter-Typ und Testbeispielen.
  - Wortliste: bestimmte Wörter (Firmen, Kunden, Projekte), wahlweise mit
    Groß-/Kleinschreibung und nur ganze Wörter. Dazu eigene Listen für Kunden, Vornamen
    und Nachnamen mit den Platzhaltern `kunde`, `vorname`, `nachname` (ADR-0021).
  - Allowlist für Werte, die nie ersetzt werden.
- Die Wortlisten (allgemein, Kunden, Vornamen, Nachnamen) sind selbst vertraulich:
  verschlüsselt speichern, im Frontend maskiert anzeigen, nie loggen. Ausnahme (neben dem
  Aufdecken nach ADR-0018): „Bearbeiten“ lädt Kunden/Vornamen/Nachnamen auf Klick im
  Klartext (ADR-0021). Der YAML-Export warnt vor dem Committen.
- Jede Sitzung hält ihre Konfigurationsversion fest. Neue Regeln gelten erst für neue
  Sitzungen, sonst ändern sich alte Nachrichten und der Prompt-Cache bricht.

Beispiel-Schema:
```yaml
version: 1
builtin:
  email: true
  api_keys: true
  jwt: true
  connection_urls: true
  ip_addresses: false
  private_keys: true
custom:
  - name: kundennummer
    pattern: 'KD-\d{6}'
    placeholder: customer_id
    examples: ["KD-123456"]
words:
  case_sensitive: false
  whole_word: true
  list: ["Projekt Falke", "Muster GmbH"]
allowlist: ["noreply@github.com"]
```

## Nachvollziehbarkeit
- Jede Anfrage bekommt eine Trace-ID. Pro Stufe speicherst du: Dauer, Tokens
  vorher/nachher, Hash von Ein- und Ausgabe, einen Diff der redigierten Fassung und
  die verwendete Konfigurationsversion.
- Das Frontend zeigt pro Anfrage eine Timeline aller Stufen mit Vorher/Nachher-Diff.

## Tests
- Tabellengetriebene Unit-Tests pro Stufe, Golden-Files unter `testdata/` für jede
  Kompressionsregel.
- Fuzz-Tests mit `testing.F`: Redact und anschließendes Rehydrate ergeben exakt die
  Ausgangsdaten. Jede Transformation ist deterministisch.
- Integrationstests mit `httptest` als Mock-Upstream, inklusive SSE-Streams, bei denen
  Platzhalter über Chunk-Grenzen verteilt sind.
- Cache-Test: Mehrere Turns hintereinander erzeugen identische Präfixe, auch nach
  einer Konfigurationsänderung während einer laufenden Sitzung.
- Konfigurationstests: YAML-Import und -Export ergeben dieselbe Konfiguration,
  ungültiges YAML wird mit verständlicher Fehlermeldung abgelehnt.
- Paritätstest zwischen Rust- und Go-Redactor (siehe oben).
- End-to-End mit aufgezeichneten, bereinigten Fixtures aus `testdata/fixtures/`.
- `go test -race ./...` muss grün sein. Ein Befehl startet alle Tests inklusive
  Frontend (`make test`), dazu eine CI-Konfiguration.

## Frontend
Technik:
- Svelte mit Vite und TypeScript, statischer Build.
- TypeScript-Typen werden aus den Go-Structs generiert (z. B. mit `tygo`), nie von
  Hand gepflegt.
- Der Build wird per `embed` ins tollgate-Binary eingebettet, ein Deployment-Artefakt.
- Live-Daten per SSE, YAML-Editor mit CodeMirror und Schema-Validierung.
- Responsiv, damit die Oberfläche auch auf dem iPhone im Tailnet nutzbar ist.

Funktionen:
- **Live:** Liste der Anfragen mit Status, Modell, Tokens und Ersparnis.
- **Anfragen:** Detailansicht mit Stufen-Timeline, Diff pro Stufe und Rückweg.
- **Kosten:** Tokens pro Tag, Client und Modell, Cache-Trefferquote, gesparte Tokens.
- **Filter:** Redactor-Konfiguration mit Liste, Editor, Wortliste (maskiert),
  Allowlist, Live-Test (Text einfügen, Treffer sehen), Versionshistorie mit Rollback
  sowie YAML-Import und -Export.
- **Kompression:** Regeln pro Befehl mit Playground, der das Ergebnis jeder Stufe zeigt.
- **Clients:** Keys erzeugen, sperren und rotieren, mit Setup-Snippet pro Client.
- **Accounts:** aktiver Account, Health von tollgate und CLIProxyAPI.
- Zugriff nur aus dem Tailnet, zusätzlich mit Admin-Token.

Design inspiriert von Things 3, ohne dessen Logos, Icons, Namen oder Assets zu
übernehmen:
- Hell und ruhig, viel Weißraum, große klare Titel, dezente Typografie.
- Schmale Sidebar links mit Icons für die Bereiche oben.
- Ein einziger dezenter Blauton als Akzent, sonst Grautöne. Keine schweren Rahmen.
- Runde Statusindikatoren statt greller Badges, sanfte und kurze Übergänge.
- Detailansichten klappen inline auf, statt neue Seiten zu öffnen.
- Vollständige Tastatursteuerung und ein sauberer Dark Mode.

## Phasen
1. **Durchreichen und messen:** Proxy, Store, Client-Keys und Frontend mit Live-Liste,
   Kosten und Clients.
2. **Redactor:** Go-Port mit Paritätstest, Hin- und Rückweg mit Streaming,
   Filter-Bereich mit YAML-Import und -Export, Detailansicht mit Diffs.
3. **Kompression:** Regeln für die Befehle mit den meisten Tokens laut Log aus
   Phase 1, plus Headroom-Port und Regel-Playground.
4. **Ausbau:** Budget-Alarm, OpenTelemetry, Feinschliff am Frontend.

Abnahme jeder Phase: Claude Code arbeitet normal über tollgate, nur mit gesetzter URL
und Client-Key. Alle Tests sind grün, und die Cache-Trefferquote ist nicht schlechter
als ohne tollgate.

<!-- gitbutler-agent-setup:start -->
## Version control

- Use GitButler (`but`) for version-control inspection and write operations, including status, diffs, branching, committing, pushing, and history edits.
- Assume multiple agents may be working in this repository. Do not move, amend, squash, discard, commit, push, or otherwise modify another agent's work unless the user asks.
- For commit just/only/specific changes on a new branch (selected-change requests), use the two-command fast path from the GitButler skill: `but diff`, then `but commit -b <branch> -m "message" <id> <id>`.
- For that fast path, after the commit succeeds, stop and summarize; do not run separate branch, staging, status, or diff commands unless the commit output is missing information you need.
- Use the installed GitButler skill for command recipes and syntax before guessing flags, using `--help`, or translating Git habits directly.
- Mutation commands report their result without appending workspace status. Add `--status-after` only when the next step needs resulting workspace IDs or details; otherwise do not rerun status or diff to verify success.
- Use a dedicated GitButler branch for each agent session and each writing subagent, following the local collaboration rules below. Commit only changes that belong to that agent.
- Do not push or open pull requests unless the user asks.
- Keep commit messages and pull request descriptions succinct: explain what changed, why it changed, and any important decision.

### Amend local fixes into the right commits

- For small cleanup or follow-up fixes, amend an unpublished local commit when the change clearly belongs with that commit's intent.
- Do not create tiny fixup commits unless the user asks.
- Use GitButler to move the relevant changes into the commit where they belong.
- Ask before rewriting pushed, reviewed, shared, or ambiguous history.

### Split unrelated changes into separate commits

- If one file contains unrelated changes, split them by hunk instead of committing the whole file.
- Keep tests with the behavior they verify.
- Split generated output, docs-only edits, or mechanical cleanup into separate commits when each commit remains coherent on its own.
- If the split is ambiguous, summarize the options before committing.
<!-- gitbutler-agent-setup:end -->

## Lokale Zusammenarbeit mit Subagenten

- Lokale Subagenten arbeiten im gemeinsamen GitButler-Workspace dieses Repos auf
  eigenen, parallel angewendeten GitButler-Branches. Keine neuen Git-Worktrees,
  separaten Checkouts oder Worktree-Isolation beim Start eines Subagenten verwenden.
  Diese Repo-Regel gilt auch, wenn ein allgemeiner Skill Worktrees empfiehlt.
- Der koordinierende Agent weist jedem schreibenden Subagenten einen eindeutigen
  Branch (z. B. `issue-<N>/<teilaufgabe>`) und klare Datei-/Modulverantwortung zu und
  übergibt diese Regeln im Auftrag. Rein lesende Subagenten benötigen keinen Branch.
  Bei Bedarf entsteht der Branch mit dem ersten gezielten
  `but commit -b <branch> -m "message" <change-id>...`.
- Alle Agenten bleiben im selben Workspace; kein `git checkout`, `git switch`,
  `but branch new --switch`, `git stash` oder `but teardown` während paralleler Arbeit.
  Vorhandene Worktrees nicht automatisch entfernen oder migrieren.
- Das Dateisystem, uncommittete Änderungen, Dependencies, Build-Ausgaben und laufende
  Server sind gemeinsam. Branches isolieren diese Zustände nicht. Überlappende Dateien
  und generierte Ausgaben vor dem Schreiben koordinieren; konkurrierende Änderungen
  an denselben Stellen nacheinander bearbeiten. Fremde Änderungen niemals zurücksetzen.
- Der koordinierende Agent serialisiert GitButler-Schreiboperationen: entweder führt
  er sie selbst aus, oder er weist sie jeweils genau einem Subagenten zu. Währenddessen
  pausieren Änderungen an den betroffenen Dateien. Change-IDs unmittelbar zuvor mit
  `but diff` ermitteln und nur eigene Dateien/Hunks auf den ausdrücklich genannten
  Branch committen; im gemeinsamen Workspace niemals alle Änderungen ohne IDs committen.
- Unabhängige Aufgaben bleiben auf parallelen Branches. Tatsächliche Abhängigkeiten
  ausdrücklich koordinieren und bei Bedarf mit `but move <kind> --above <basis>`
  stapeln. Die GitHub-Issue-Abhängigkeiten und der OpenSpec-Ablauf bleiben verbindlich.
- Tests im gemeinsamen Workspace prüfen die Kombination aller aktiven Branches.
  Vor einer unabhängigen Auslieferung auch die Eigenständigkeit des jeweiligen Branches
  prüfen. Workspace-weite Änderungen, History-Edits und Bereinigung erst durchführen,
  wenn alle beteiligten Agenten ihre Arbeit abgestimmt oder pausiert haben.
- Wenn Aufgaben inkompatible Checkout- oder Laufzeitzustände benötigen, die betroffenen
  Aufgaben nacheinander ausführen; nicht automatisch auf Worktrees ausweichen.

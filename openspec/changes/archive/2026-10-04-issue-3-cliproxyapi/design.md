# Design

## Context

ADR-0001 legt fest: CLIProxyAPI (CPA) läuft als eigener Prozess nur auf `127.0.0.1`, die Kopie ist ein eigenes Go-Modul, die Pflichtkonfiguration liegt als Vorlage unter `deploy/cliproxyapi/`. #2 ist gemergt, `third_party/cliproxyapi/.gitkeep` und `deploy/cliproxyapi/.gitkeep` waren Platzhalter.

## Goals / Non-Goals

**Goals:** Herkunft und Lizenz belegt, Pflichtkonfiguration reproduzierbar, kein veröffentlichter Port, ein Client, der den Health-Check gegen das echte Binary besteht.

**Non-Goals:** siehe `proposal.md`, Abschnitt "Was ausdrücklich nicht".

## Decisions

1. **Kopie per `git archive` des festen Commits.** Kein Submodul, kein `.git`. Zwei Abweichungen von der Dateiliste (Upstream-`AGENTS.md`/`CLAUDE.md`, von der geschachtelten `.gitignore` verdeckte Dateien) stehen begründet in `PATCHES.md`. Codepatches gibt es nicht. Die geschachtelte `.gitignore` bleibt als Schutz vor versehentlich committeten `config.yaml`/`auths/`.
2. **Vorlage mit Platzhaltern, Rendern per `sh`-Skript.** CPA kennt keine Datei- oder Umgebungsersetzung für `access.api-keys` (nur `MANAGEMENT_PASSWORD` für das Management-Secret). Deshalb enthält `config.yaml` die Platzhalter `@TOLLGATE_API_KEY@`, `@MANAGEMENT_SECRET@`, `@AUTH_DIR@`, und `render-config.sh` ersetzt sie aus Secret-Dateien (`*_FILE`, nie aus Argumenten). Geheimnisse müssen `[A-Za-z0-9_-]` und mindestens 32 Zeichen lang sein; so bleiben `sed` und YAML-Quoting trivial, und schwache oder leere Schlüssel (die den Zugang öffnen würden) werden abgelehnt. Die Ausgabe entsteht mit `umask 077`.
3. **Pflichtwerte genau nach ADR-0001, Punkt 4** (`server.host`, `access.api-keys`, `requests.passthrough-headers`, `upstream.claude.disable-claude-cloak-mode`, `observability.logs.request-log`, `management.secret-key`, `allow-remote`, `disable-control-panel`) plus `server.port` und `oauth.auth-dir`. Die Schlüsselpfade wurden gegen `config.example.yaml` @ `8ef43e4` geprüft (v8-Layout).
4. **Compose: nur der Dienst `cliproxyapi`, ohne `ports:`.** CPA bindet im Container `127.0.0.1`; andere Dienste erreichen es über `network_mode: "service:cliproxyapi"` (gemeinsamer Netzwerk-Namensraum). Ob tollgate dort oder im Host-Netzwerk bzw. mit Tailscale-Sidecar läuft, entscheidet #11 (ARCHITECTURE §5); ein tollgate-Dienst entsteht dort, weil es noch kein tollgate-Image gibt. Das Image baut mit eigenem `Dockerfile` und `CGO_ENABLED=0` (ADR-0001, Punkt 2) statt mit dem Upstream-Dockerfile (cgo, `EXPOSE`, Port-Mappings für OAuth-Callbacks). Der Quellbaum kommt über `additional_contexts` (`cpa-src`) herein, damit Vorlage und Skript im Build-Kontext liegen. Secrets laufen über Compose-`secrets:` (Dateien unter `secrets/`).
5. **auth-dir als benanntes Volume `/data/auth`.** Die Regel "nur die Token-Datei des aktiven Accounts" wird durch #10 geprüft (Health-Fehler bei mehr als einer Datei); hier wird nur ein eigener, leerer Ort bereitgestellt.
6. **`internal/upstream.Client`: nur `New` und `Health`.** `New` akzeptiert ausschließlich `http://` mit Loopback-Host (`localhost`, `127.0.0.0/8`, `::1`), ohne Pfad und Userinfo; alles andere ist ein Fehler (harte Regel: CPA nur localhost, und kein Schlüssel darf an ein fremdes Ziel gehen). `Health` ruft `GET /healthz` (öffentlich, ohne Schlüssel), verlangt 200 und `{"status":"ok"}`, folgt keinen Weiterleitungen und gibt den Antwortkörper nie in Fehlern aus. Der Management-Endpunkt `/v8/management/credentials` kommt mit #10 in den Client.
7. **Smoke-Test gegen das echte Binary.** `TestSmokeRealCLIProxyAPI` läuft nur mit `TOLLGATE_CPA_BIN`; ohne gebautes Binary wird er übersprungen, damit `make test` ohne CPA-Build grün bleibt. Er rendert die Vorlage in ein Temp-Verzeichnis, startet CPA auf einem freien Port, wartet auf den Health-Check, prüft die Management-Authentifizierung (401 ohne Schlüssel, 401 mit dem tollgate-Schlüssel, 200 mit dem Management-Schlüssel) und dass keine Nicht-Loopback-Adresse des Hosts den Port erreicht. Er entspricht Schritt 4 des Update-Ablaufs aus ADR-0001.

## Risks / Trade-offs

- Die Kopie ist groß (rund 31 MB, 1.800 Dateien). Rückfalloption laut ADR: gepinntes Image ohne Kopie.
- Das Image wurde im Rahmen des Changes nicht gebaut (nur `docker compose config`); der Go-Build der Kopie mit `CGO_ENABLED=0` ist verifiziert.
- Der Smoke-Test ist ohne Binary übersprungen; CI führt ihn vorerst nicht aus (CI-Änderung gehört nicht zu diesem Change).

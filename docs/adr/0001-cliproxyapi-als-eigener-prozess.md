# ADR-0001: CLIProxyAPI als eigener Prozess

- **Status:** Angenommen (Freigabe Niclas, 2026-10-04, PR #26); teilweise abgelöst durch [ADR-0014](0014-cliproxyapi-gepinnt-ohne-kopie.md) (Punkte 2 und 3)
- **Datum:** 2026-10-04
- **Issue:** #1

## Kontext

tollgate leitet an CLIProxyAPI (CPA) weiter. CPA hält das OAuth-Token des Claude-Accounts
und spricht mit Anthropic. Der Auftrag verlangt eine Kopie unter `third_party/cliproxyapi`
und bevorzugt die Lösung, bei der Upstream-Updates einfach bleiben.

CPA bietet beide Wege an, als Bibliothek und als eigenen Prozess.

**Als Bibliothek** gibt es ein SDK unter `sdk/cliproxy`:
`cliproxy.NewBuilder().WithConfig(…).Build()` und danach `svc.Run(ctx)`. Ein Beispiel liegt
unter `examples/custom-provider/main.go`. Dagegen sprechen vier Punkte:
- Die meisten SDK-Typen sind nur Aliase auf `internal/`, z. B. `sdk/config/config.go` mit
  `type Config = internalconfig.Config`.
- Das gesamte Claude-Verhalten liegt in `internal/` und ist von außen weder importierbar noch
  änderbar. Das betrifft Client-Erkennung, Cloaking, Beta-Header, CCH-Signatur und Executor.
- Es gibt keine Stabilitätszusage. Das Modul steht schon bei `/v8`, `docs/sdk-usage.md`
  beschreibt noch `/v6` und importiert `internal/config`, was von außen nicht kompiliert.
- `sdk/cliproxy` zieht über `internal/api` schwere Abhängigkeiten nach: pion/webrtc,
  go-redis, refraction-networking/utls und gin.

**Als eigener Prozess** läuft CPA mit `-config <pfad>`. Bindung, Client-Keys, auth-dir,
Logging und Management-API sind über `config.example.yaml` steuerbar.

**Verhalten gegenüber Claude Code.** CPA prüft, ob ein Request von Claude Code kommt
(`internal/runtime/executor/helps/claude_client_detection.go`). Als bestätigt gilt er nur,
wenn alles zutrifft:
- `X-App: cli`
- User-Agent `claude-cli/<major.minor>.<patch>` in derselben Minor-Linie wie die Baseline
  `claude-cli/2.1.280`, mit Patch ≥ 280 (`helps/claude_device_profile.go`)
- `anthropic-beta` enthält `claude-code-20250219`
- `metadata.user_id` ist gültig

Ist der Request nicht bestätigt und der Account ein OAuth-Account, greift das Cloaking
(`claude_executor_cloaking.go`). CPA ersetzt dann den System-Prompt, baut die Betas neu auf und
hebt die TTLs von `cache_control` an. Das bricht den Prompt-Cache.

Auch bei bestätigten Requests ändert CPA Bytes:
- Es baut `metadata.user_id` aus der Identität des Accounts neu.
- Es signiert den CCH im Billing-Block neu (`claude_signing.go`).
- Es ergänzt `oauth-2025-04-20` in den Betas.

Diese Änderungen sind deterministisch pro Account und für den Cache unkritisch.

## Entscheidung

1. **CPA läuft als eigener Prozess und ist nur an `127.0.0.1` gebunden.** tollgate spricht
   mit CPA ausschließlich über HTTP.
2. **Kopie als eigenständiges Go-Modul.** Der Code liegt unter `third_party/cliproxyapi/` mit
   eigenem `go.mod` und wird von tollgate nie importiert. `go test ./...` im tollgate-Modul
   ignoriert verschachtelte Module automatisch. Gebaut wird ein eigenes Binary mit
   `CGO_ENABLED=0`; nur der Plugin-Loader braucht cgo, und dafür gibt es einen `!cgo`-Stub.
3. **Keine Patches als Ziel.** `third_party/cliproxyapi/PATCHES.md` hält den Upstream-Commit
   fest und begründet jede Abweichung. Ein Upstream-Update läuft so ab:
   1. Kopie austauschen.
   2. Patches erneut anwenden.
   3. `config.example.yaml` gegen die Vorlage vergleichen.
   4. Smoke-Test laufen lassen.
4. **Pflichtkonfiguration für CPA.** Die Vorlage liegt unter `deploy/cliproxyapi/config.yaml`
   und enthält keine Geheimnisse:

   | Schlüssel | Wert | Grund |
   |---|---|---|
   | `server.host` | `127.0.0.1` | harte Regel: CPA nur an localhost |
   | `access.api-keys` | genau ein starker Key für tollgate (aus Datei) | Eine leere Liste öffnet den Zugang (`sdk/access/manager.go`). |
   | `requests.passthrough-headers` | `true` | `anthropic-ratelimit-*` und `request-id` erreichen Claude Code. |
   | `upstream.claude.disable-claude-cloak-mode` | `true` | Sicherheitsnetz gegen das Umschreiben des System-Prompts |
   | `observability.logs.request-log` | `false` | Kein Request-Body landet im CPA-Log. |
   | `management.secret-key` | gesetzt (aus Datei) | Health und aktiver Account für tollgate |
   | `management.allow-remote` | `false` | Management nur über localhost |
   | `management.disable-control-panel` | `true` | kein nachgeladenes Web-Panel |

5. **Header- und Body-Regeln für tollgate beim Weiterleiten:**
   - Unverändert durchreichen: `User-Agent`, `X-App`, `anthropic-version`, `anthropic-beta`,
     alle `X-Claude-Code-*` und alle `X-Stainless-*`.
   - `metadata.user_id` und den Billing-Block in `system[0]` nie verändern.
   - Ersetzen: `Authorization` und `x-api-key` durch den CPA-Key von tollgate.
   - Hop-by-hop-Header entfernen, `Content-Length` neu berechnen.
6. **Aktiver Account.** Im auth-dir liegt laut harter Regel genau eine Token-Datei
   (`claude-<email>.json`). tollgate fragt `GET /v8/management/credentials` ab
   (`internal/api/server_management_v8.go`) und zeigt E-Mail, Status und Cooldowns an.
   Liegt mehr als eine Datei im auth-dir, meldet tollgate einen Fehler im Health-Check.
7. **Baseline-Wächter.** tollgate vergleicht den User-Agent eingehender Claude-Code-Requests
   mit der CPA-Baseline. Liegt er außerhalb, erscheint im Frontend eine Warnung, z. B. bei
   einem Wechsel auf 2.2.x. Abhilfe: `upstream.claude.header-defaults.user-agent` anpassen
   oder CPA aktualisieren.

## Alternativen

- **SDK-Einbettung** (ein Binary, `WithSelectedAuthIDCallback` liefert den Account pro
  Request): verworfen. Sie bringt keine zusätzliche Kontrolle über das Claude-Verhalten, dafür
  schwere Abhängigkeiten und keine API-Stabilität. Jedes Upstream-Update kann den Build brechen.
- **Fork mit Änderungen in `internal/`:** verworfen. Die Updates würden dauerhaft teuer.
- **Nur ein gepinntes Docker-Image (`eceasy/cli-proxy-api:<tag>`) ohne Kopie:** am einfachsten
  zu aktualisieren, widerspricht aber dem Auftrag „Kopie unter third_party“. Bleibt als
  Rückfalloption, falls die Kopie (rund 33 MB, 1.600 Go-Dateien) im öffentlichen Repo stört.

## Konsequenzen

- tollgate bleibt ein kleiner Reverse-Proxy ohne CPA-Abhängigkeiten im eigenen `go.mod`.
- Ein zusätzlicher Hop über localhost, vernachlässigbar.
- Es gibt keine Account-ID pro Request. Das ist unkritisch, weil es nur einen aktiven
  Account gibt.
- `make build` baut zwei Binaries. `docker-compose.yml` hat zwei Dienste, CPA ohne Port nach
  außen.
- **#3** setzt die Kopie mit `NOTICE.md`, `PATCHES.md`, die Konfigurationsvorlage und
  docker-compose um.
- **#6** setzt die Header-Regeln aus Punkt 5 um.
- **#10** zeigt den aktiven Account und die Baseline-Warnung an.

## Quellen

CLIProxyAPI @ `8ef43e4`:
- `sdk/cliproxy/builder.go`, `sdk/config/config.go`, `examples/custom-provider/main.go`, `docs/sdk-usage.md`
- `internal/runtime/executor/helps/claude_client_detection.go`, `helps/claude_device_profile.go`
- `internal/runtime/executor/claude_executor_cloaking.go`, `claude_signing.go`
- `config.example.yaml` (`server.host`, `access.api-keys`, `requests.passthrough-headers`, `upstream.claude.disable-claude-cloak-mode`, `management.*`, `oauth.auth-dir`)
- `internal/api/server_management_v8.go` (`/v8/management/credentials`)
- `sdk/access/manager.go`, `internal/auth/claude/filename.go`
- `LICENSE` (MIT; Copyright Luis Pater und Router-For.ME)

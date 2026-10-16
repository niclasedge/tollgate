# Proposal

Issue: [#19 Compress-Stage: Regeln pro Bash-Befehl](https://github.com/niclasedge/tollgate/issues/19)

> **Gestapelt auf #18** (Token-Analyse). Dessen Capability `compression` ist noch nicht
> archiviert; dieser Change ergänzt sie als neue Requirements.

## Why

Bash-Ausgaben sind 65 % der Tool-Bytes (docs/token-analyse.md). tollgate soll sie im Proxy
kürzen und damit rtk auf den Clients ersetzen, ohne den Prompt-Cache zu brechen (ADR-0007).

## What Changes

- Neues Paket `internal/compress` mit der Stufe `compress` zwischen Redact und Meter.
- Zustandslos über den ganzen Verlauf: jedes `tool_result`, dessen `tool_use_id` auf einen
  `tool_use` mit `name == "Bash"` zeigt, läuft durch dieselbe reine Funktion.
- Befehlserkennung (`Normalize`): Setup-Segmente (`cd`, `pushd`, `export`, `set`),
  Env-Zuweisungen, `time`/`env`, harmlose Umleitungen, `git --no-pager` und
  Verzeichnis-Optionen fallen weg. Pipes, `||`, Befehlssubstitution, Heredocs, offene
  Anführungszeichen und mehr als ein verbleibender Befehl → keine Kompression.
- Regeln: konfigurierte Regeln (Abschnitt `compress:` der gepinnten Version) vor den
  eingebauten, die erste passende gewinnt; `passthrough` schaltet einen Befehl ab.
- Gates: Ausgabe ≥ `min_bytes` (Standard 2048) und Ergebnis samt Kennzeichnungszeile strikt
  kleiner, sonst bleibt das Original.
- Kennzeichnungszeile `[tollgate: <regel> · <n> Zeilen ausgelassen · <details>]`.
- Trace: eine Zeile je komprimierter Ausgabe mit Pfad, Regel und Größen, nie Ausgabetext.
- `jsonsplice.Target` trägt `tool_id` und `tool_name`, damit `tool_result` und `tool_use`
  ohne zweiten Parse zusammenfinden.
- Schalter `compress.enabled` / `TOLLGATE_COMPRESS_ENABLED`, im Binary standardmäßig aus,
  in `docker-compose.yml` mit Standard `true` durchgereicht, damit `just setup` immer den
  aktuellen Stand mit Kompression startet.
- Golden-Test-Gerüst `compresstest.Golden` für `testdata/<regel>/*.input` → `*.golden`,
  `-update`, mit Determinismusprüfung.

## Capabilities

### Modified Capabilities

- `compression`: Compress-Stufe, Befehlserkennung, Gates, Kennzeichnung, Schalter.

## Impact

- Go: `internal/compress` (`command.go`, `config.go`, `compress.go`, `stage.go`, `strat/`,
  `compresstest/`), `internal/jsonsplice/scan.go`, `internal/redactcfg` (`CompressConfig`,
  `Manager.Compressor`), `internal/config` (Schalter), `cmd/tollgate` (Verdrahtung).
- Deploy: `docker-compose.yml`, `deploy/compose_env_test.go`, `docs/DEPLOY.md`.
- Einschalten kostet einmal einen Cache-Miss je laufender Sitzung, danach ist der Präfix
  wieder stabil.

## Was ausdrücklich nicht

- Keine Strategien außer `passthrough` (#20).
- Keine Regel für Dateilesen per Bash (`cat`, `sed -n`, `head`, `tail`).
- Kein Aufteilen von Pipelines: deren Ausgabe ist meist schon mit `head`/`tail` gekürzt.

# Tasks

Gestapelt auf #18.

## 1. Kern

- [x] 1.1 Tests zuerst: `Normalize` (Ketten, Env, Umleitungen, abgelehnte Formen), Regelreihenfolge, Gates, Kennzeichnung; rot sehen.
- [x] 1.2 `jsonsplice.Target` um `tool_id` und `tool_name` ergänzen, mit Test.
- [x] 1.3 `command.go`, `config.go`, `compress.go`, `strat/`, `compresstest/`.
- [x] 1.4 `stage.go`: nur `messages` und `count_tokens`, nur Bash-Ergebnisse, Bericht ohne Ausgabetext; Tests für Read, Edit, Write, Grep, Glob, MCP und unbekannte Tools.

## 2. Konfiguration und Verdrahtung

- [x] 2.1 `redactcfg`: `Document.CompressConfig`, `Manager.Compressor` für die gepinnte Version.
- [x] 2.2 `config`: Schalter `compress.enabled` / `TOLLGATE_COMPRESS_ENABLED`, Standard aus.
- [x] 2.3 `cmd/tollgate`: Stufe nach Redact einhängen, Test an und aus.
- [x] 2.4 `docker-compose.yml`, Compose-Test, `docs/DEPLOY.md`, Schema-Beschreibung.

## 3. Abschluss

- [x] 3.1 `make test`, `make lint` grün.

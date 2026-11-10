# Proposal

Issue: [#4 CPA-Pin auf v8.0.63 anheben, Katalog-Overlay entfernen](https://github.com/niclasedge/tollgate/issues/4)

## Why

Der Pin `8ef43e4` kennt `claude-haiku-5-5` und `claude-fable-5-2` nicht; beide kommen über das Build-Overlay aus ADR-0020. v8.0.63 bettet beide ein. Jedes weitere neue Modell bräuchte sonst einen eigenen Patch.

## What Changes

- `make cpa` baut wie das Image in einer Modulkopie (Upstream hat jetzt ein `replace`).
- `ARG CPA_VERSION` auf `6db7d4b` (v8.0.63); `UPSTREAM.md`, `NOTICE.md`, ADR-Index nachgezogen.
- Overlay, Overlay-Test, `apply.sh`-Zeilen, Dockerfile-Test und `third_party/cpa-model-overlay/` entfallen.
- `config.yaml`: `upstream.claude.header-defaults` auf die alte Baseline `claude-cli/2.1.280`, `server.kv-store.disabled: true`.
- ADR-0022 hält Prüfergebnis und Entscheidung fest und löst ADR-0020 ab.

## Impact

- Capability `cpa-model-overlay` entfällt.
- Egress, Routing und Refresh-Barriere bleiben; Thinking-Binding von CPA kostet einen Cache-Miss pro Konversation (ADR-0022).

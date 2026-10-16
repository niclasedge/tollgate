## Why
Reviewbefund aus #28: Hintergrund-Refresh kann zwischen Snapshot und Delete den gesicherten Token ungültig machen. Issue: https://github.com/niclasedge/tollgate/issues/77.
## What Changes
Begrenzte authentifizierte CPA-Refresh-Barriere gemäß ADR-0016, Quellen nur im temporären Build, Nachweis am vorhandenen Refresh-Mutex.
## Capabilities
### New Capabilities
- `cpa-refresh-barrier`: Pause/Resume mit gesichertem Snapshot.
## Impact
Branch `issue-77/cpa-refresh-barriere`, gemeinsam mit angewendetem #72. Dateien: `deploy/cliproxyapi/Dockerfile`, `deploy/cliproxyapi/bridge/*`, `deploy/cliproxyapi/UPSTREAM.md`, `docs/adr/0016-cpa-refresh-barriere.md`, `third_party/NOTICE.md`, `third_party/cpa-refresh-barrier/PATCHES.md`, `internal/upstream/pin_test.go`, dieser Change. Kein Upstream-Vendoring.

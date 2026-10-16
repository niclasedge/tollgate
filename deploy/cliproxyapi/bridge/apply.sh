#!/bin/sh
# Applies the authored extension only in an ephemeral upstream build directory.
# Fail closed if the pinned source no longer has exactly the expected seams.
set -eu
src=${1:?upstream source directory}
bridge=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cp "$bridge/refresh_pause.go.txt" "$src/sdk/cliproxy/auth/tollgate_refresh_pause.go"
cp "$bridge/refresh_pause_test.go.txt" "$src/sdk/cliproxy/auth/tollgate_refresh_pause_test.go"
cp "$bridge/management_pause.go.txt" "$src/internal/api/handlers/management/tollgate_refresh_pause.go"
# ADR-0020: catalog overlay for models the pinned embedded catalog lacks.
cp "$bridge/models_overlay.go.txt" "$src/internal/registry/tollgate_models_overlay.go"
cp "$bridge/models_overlay_test.go.txt" "$src/internal/registry/tollgate_models_overlay_test.go"
file=$src/sdk/cliproxy/auth/conductor_refresh.go
awk '/defer lock.mu.Unlock\(\)/ { n++; print; print "\tif m.tollgateRefreshPaused(id) { return nil, errors.New(\"credential refresh paused\") }"; next } {print} END {if(n!=1) exit 42}' "$file" > "$file.tmp"
mv "$file.tmp" "$file"
file=$src/internal/api/server_management_v8.go
awk '/v8.POST\("\/credentials\/refresh",/ { n++; print; print "\tv8.POST(\"/credentials/refresh-pause\", s.mgmt.TollgateRefreshPause)"; next } {print} END {if(n!=1) exit 42}' "$file" > "$file.tmp"
mv "$file.tmp" "$file"

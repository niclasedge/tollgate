#!/usr/bin/env bash
# Smoke test of the container stack without a Tailnet (`just smoke`).
# Builds the images, starts CLIProxyAPI and tollgate with throwaway secrets
# (deploy/compose.smoke.yml), checks the routes and removes everything again.
# No Claude login: the proxied call only has to reach CLIProxyAPI.
set -euo pipefail

cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
export TOLLGATE_SECRETS_DIR=$tmp
export COMPOSE_PROJECT_NAME=tollgate-smoke
port=${TOLLGATE_SMOKE_PORT:-18443}
export TOLLGATE_SMOKE_PORT=$port
# Only for interpolation; compose.smoke.yml replaces the Tailnet port.
export TOLLGATE_HOST_IP=127.0.0.1
compose=(docker compose -f docker-compose.yml -f deploy/compose.smoke.yml)
base=http://127.0.0.1:$port

cleanup() {
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$tmp"
}
trap cleanup EXIT

fail() {
  echo "smoke: FAIL: $*" >&2
  "${compose[@]}" logs --no-color --tail 40 >&2 || true
  exit 1
}

for s in tollgate_api_key management_secret admin_token; do
  head -c 48 /dev/urandom | base64 | tr -d '=+/\n' > "$tmp/$s"
done
# Slack stays off: the notifier discards the spool (ADR-0019).
: > "$tmp/slack_bot_token"
: > "$tmp/slack_channel"
chmod 644 "$tmp"/*

"${compose[@]}" up -d --build --wait --wait-timeout 300 || fail "stack did not become healthy"

code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

[ "$(code "$base/")" = 200 ] || fail "GET / is not 200"
curl -s "$base/" | grep -q '<html' || fail "GET / is not the frontend"
[ "$(code "$base/v1/models")" = 401 ] || fail "request without client key is not 401"
[ "$(code "$base/api/clients")" = 401 ] || fail "admin API without token is not 401"

admin=$(cat "$tmp/admin_token")
resp=$(curl -s -H "Authorization: Bearer $admin" -d '{"name":"smoke"}' "$base/api/clients")
key=$(printf '%s' "$resp" | sed -n 's/.*"key":"\([^"]*\)".*/\1/p')
[ -n "$key" ] || fail "creating a client failed"

# The client key passes tollgate; without a Claude login CLIProxyAPI answers on
# its own, but never with tollgate's 401.
models=$(code -H "Authorization: Bearer $key" "$base/v1/models")
[ "$models" != 401 ] && [ "$models" != 000 ] || fail "proxied request got $models"

# shellcheck disable=SC2016 # expanded by the shell inside the container
"${compose[@]}" exec -T tollgate sh -c 'test "$(stat -c %a /run/tollgate/admin.token)" = 600' \
  || fail "admin token copy is not 0600"
# The entrypoint execs tollgate, so it is PID 1 of the container.
user=$("${compose[@]}" exec -T tollgate stat -c %U /proc/1 | tr -d '[:space:]')
[ "$user" = tollgate ] || fail "tollgate does not run as user tollgate ($user)"

# Production binding mode: container address plus a Tailnet published address.
pub=$("${compose[@]}" exec -T tollgate cat /run/tollgate/published | tr -d '[:space:]')
[ "$pub" = 100.64.0.1:8443 ] || fail "published address is $pub"
"${compose[@]}" logs --no-color tollgate | grep -q 'dev-allow-any-listen' && fail "tollgate runs with the dev flag"

# Egress of the shared namespace (ADR-0017): Anthropic yes, anything else no.
# tollgate inherits the rules of the cliproxyapi namespace.
egress() { "${compose[@]}" exec -T tollgate timeout 8 bash -c "exec 3<>/dev/tcp/$1/443" >/dev/null 2>&1; }
egress api.anthropic.com || fail "namespace cannot reach api.anthropic.com:443"
egress example.com && fail "namespace reaches example.com:443 (egress not limited)"
cpa_user=$("${compose[@]}" exec -T cliproxyapi stat -c %U /proc/1 | tr -d '[:space:]')
[ "$cpa_user" = cpa ] || fail "CLIProxyAPI does not run as user cpa ($cpa_user)"
"${compose[@]}" exec -T cliproxyapi grep -q '^CapEff:[[:space:]]*0*$' /proc/1/status \
  || fail "CLIProxyAPI keeps capabilities"
"${compose[@]}" exec -T cliproxyapi sh -c 'tr "\0" " " < /proc/1/cmdline' | grep -q -- '-local-model' \
  || fail "CLIProxyAPI runs without -local-model"
# Fail-closed: without NET_ADMIN the entrypoint stops at the rules.
nocap=$(docker run --rm --network none tollgate-cliproxyapi:local 2>&1) \
  && fail "CLIProxyAPI image starts without NET_ADMIN"
grep -q 'cpa-entrypoint: no usable iptables' <<<"$nocap" \
  || fail "CLIProxyAPI image without NET_ADMIN failed for another reason: $nocap"

echo "smoke: OK (frontend 200, no key 401, admin 401, client created, proxied $models, token 0600, non-root, published mode, egress only Anthropic, fail-closed)"

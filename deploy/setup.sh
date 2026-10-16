#!/usr/bin/env bash
# tollgate setup and operations (ADR-0013). Called by the justfile:
#   setup.sh [setup]          check prerequisites, create secrets, log in, deploy
#   setup.sh up               refresh the host Tailnet IP and start the stack
#   setup.sh down | logs      stop the stack / follow the logs
#   setup.sh login            OAuth login with your Claude subscription
#   setup.sh import-token F   copy an existing CLIProxyAPI token file into the auth dir
#   setup.sh client NAME      create a client key and print the setup snippet
#   setup.sh status           containers, Tailnet address, token present?
# Secrets are written to files and never printed (except a new client key,
# which exists only once).
set -euo pipefail

cd "$(dirname "$0")/.."
SECRETS=${TOLLGATE_SECRETS_DIR:-./secrets}
export TOLLGATE_SECRETS_DIR=$SECRETS
compose=(docker compose)

say() { printf '\033[1m==> %s\033[0m\n' "$*"; }
die() { printf 'setup: %s\n' "$*" >&2; exit 1; }

need_tools() {
  command -v docker >/dev/null || die "docker is missing (install Docker or OrbStack)"
  docker info >/dev/null 2>&1 || die "the Docker daemon is not running"
  docker compose version >/dev/null 2>&1 || die "docker compose v2 is missing"
  command -v curl >/dev/null || die "curl is missing"
}

# host_tailnet_ip: first IPv4 of this host in 100.64.0.0/10, from the
# tailscale CLI or, on macOS, the CLI inside the Tailscale app.
host_tailnet_ip() {
  local ts ip a b
  for ts in tailscale /Applications/Tailscale.app/Contents/MacOS/Tailscale; do
    command -v "$ts" >/dev/null 2>&1 || continue
    while read -r ip; do
      [[ $ip =~ ^([0-9]+)\.([0-9]+)\.[0-9]+\.[0-9]+$ ]] || continue
      a=${BASH_REMATCH[1]} b=${BASH_REMATCH[2]}
      if [ "$a" = 100 ] && [ "$b" -ge 64 ] && [ "$b" -le 127 ]; then
        echo "$ip"
        return 0
      fi
    done < <("$ts" ip -4 2>/dev/null || true)
  done
  return 1
}

# ensure_host_ip: write TOLLGATE_HOST_IP to .env (read by docker compose).
# Docker publishes tollgate only on this address (ADR-0013).
ensure_host_ip() {
  local ip
  ip=$(host_tailnet_ip) || die "this host has no Tailnet IPv4; start Tailscale (tailscale up) and retry"
  export TOLLGATE_HOST_IP=$ip
  if ! grep -qx "TOLLGATE_HOST_IP=$ip" .env 2>/dev/null; then
    { grep -v '^TOLLGATE_HOST_IP=' .env 2>/dev/null || true; echo "TOLLGATE_HOST_IP=$ip"; } > .env.tmp
    mv .env.tmp .env
    say "Tailnet IP of this host: $ip (saved in .env)"
  fi
}

# host_ip_soft: for commands that do not publish tollgate (login, import,
# status, down, logs). Uses the current Tailnet IP if there is one, else the
# value in .env, else a loopback stand-in that only satisfies compose
# interpolation (tollgate refuses a non-Tailnet published address anyway).
host_ip_soft() {
  local ip
  if ip=$(host_tailnet_ip); then
    ensure_host_ip
  elif ! grep -q '^TOLLGATE_HOST_IP=' .env 2>/dev/null; then
    export TOLLGATE_HOST_IP=127.0.0.1
  fi
}

# random_secret FILE: 48 random bytes, URL-safe, only if FILE does not exist.
random_secret() {
  [ -s "$1" ] && return 0
  (umask 077; head -c 48 /dev/urandom | base64 | tr -d '=+/\n' > "$1")
  say "created $(basename "$1")"
}

ensure_secrets() {
  mkdir -p "$SECRETS"
  chmod 700 "$SECRETS"
  random_secret "$SECRETS/tollgate_api_key"
  random_secret "$SECRETS/management_secret"
  random_secret "$SECRETS/admin_token"
  # Slack alerts are optional (ADR-0019): empty files switch them off;
  # `just slack-secret` fills them.
  local s
  for s in slack_bot_token slack_channel; do
    [ -e "$SECRETS/$s" ] || (umask 077; : > "$SECRETS/$s")
  done
  # Containers run with their own UIDs and read the bind-mounted files; the
  # directory (0700) keeps other host users out.
  chmod 644 "$SECRETS"/{tollgate_api_key,management_secret,admin_token,slack_bot_token,slack_channel}
}

# slack_sops_file: TOLLGATE_SLACK_SOPS_FILE from the environment or .env,
# with a leading ~ expanded; empty if not configured. The file is a
# SOPS-encrypted dotenv with SLACK_BOT_TOKEN and SLACK_ALERT_CHANNEL.
slack_sops_file() {
  local f=${TOLLGATE_SLACK_SOPS_FILE:-}
  if [ -z "$f" ] && [ -f .env ]; then
    f=$(sed -n 's/^TOLLGATE_SLACK_SOPS_FILE=//p' .env | tail -n 1)
  fi
  f=${f%\"} f=${f#\"}
  case $f in "~/"*) f=$HOME/${f#\~/} ;; esac
  printf '%s' "$f"
}

# cmd_slack: writes the Slack bot token and channel of the notifier from
# SLACK_BOT_TOKEN and SLACK_ALERT_CHANNEL. Without them it decrypts
# TOLLGATE_SLACK_SOPS_FILE with `sops exec-env`; SOPS_AGE_KEY_FILE defaults to
# ~/.config/sops/age/keys.txt (sops on macOS looks elsewhere by default).
# The values are never printed; the notifier reads them on its next run.
cmd_slack() {
  if [ -z "${SLACK_BOT_TOKEN:-}" ] && [ -z "${TOLLGATE_SLACK_VIA_SOPS:-}" ]; then
    local f
    f=$(slack_sops_file)
    [ -n "$f" ] || die "set SLACK_BOT_TOKEN and SLACK_ALERT_CHANNEL, or TOLLGATE_SLACK_SOPS_FILE in .env"
    [ -f "$f" ] || die "TOLLGATE_SLACK_SOPS_FILE not found: $f"
    command -v sops >/dev/null || die "sops is missing (needed for TOLLGATE_SLACK_SOPS_FILE)"
    SOPS_AGE_KEY_FILE=${SOPS_AGE_KEY_FILE:-$HOME/.config/sops/age/keys.txt} TOLLGATE_SLACK_VIA_SOPS=1 \
      sops exec-env "$f" 'deploy/setup.sh slack' || die "could not decrypt $f with sops"
    return
  fi
  [ -n "${SLACK_BOT_TOKEN:-}" ] || die "SLACK_BOT_TOKEN is not set"
  [ -n "${SLACK_ALERT_CHANNEL:-}" ] || die "SLACK_ALERT_CHANNEL is not set"
  ensure_secrets
  printf '%s' "$SLACK_BOT_TOKEN" > "$SECRETS/slack_bot_token"
  printf '%s' "$SLACK_ALERT_CHANNEL" > "$SECRETS/slack_channel"
  say "Slack secrets written; the notifier sends within 10 s (start it with: just up)"
}

# token_count: number of token files in the CLIProxyAPI auth volume.
token_count() {
  # shellcheck disable=SC2016 # expanded by the shell inside the container
  "${compose[@]}" --profile login run --rm --no-deps -T --entrypoint sh cpa-login \
    -c 'set -- /data/auth/*.json; [ -e "$1" ] && echo $# || echo 0'
}

# tokens: token_count, but a failed check stops the script instead of reading
# as "no token" or "token present".
tokens() {
  local n
  n=$(token_count) || die "could not inspect the auth volume; see the docker error above"
  [[ $n =~ ^[0-9]+$ ]] || die "could not inspect the auth volume (got: $n)"
  echo "$n"
}

cmd_login() {
  need_tools
  host_ip_soft
  ensure_secrets
  local n
  n=$(tokens)
  [ "$n" = 0 ] || die "the auth dir already holds a token; only one account may be active (AGENTS.md). Remove it first: docker volume rm, or wait for the account switch (#28)."
  say "Claude login: open the URL printed below in your browser and confirm."
  local tty=()
  [ -t 0 ] || tty=(-T) # e.g. over ssh without a terminal
  "${compose[@]}" --profile login run --rm --service-ports "${tty[@]}" cpa-login
  n=$(tokens)
  [ "$n" = 1 ] || die "login did not produce a token file"
  say "token saved in the auth volume"
}

cmd_import_token() {
  local file=${1:-}
  [ -f "$file" ] || die "usage: just import-token <path to CLIProxyAPI token .json>"
  need_tools
  host_ip_soft
  ensure_secrets
  local n abs name
  n=$(tokens)
  [ "$n" = 0 ] || die "the auth dir already holds a token; only one account may be active"
  abs=$(cd "$(dirname "$file")" && pwd)/$(basename "$file")
  name=$(basename "$file")
  case $name in *.json) ;; *) die "token files end in .json";; esac
  case $abs in *:*) die "the path must not contain ':' (docker -v syntax)";; esac
  # shellcheck disable=SC2016 # expanded by the shell inside the container
  "${compose[@]}" --profile login run --rm --no-deps -T --user 0:0 \
    -v "$abs:/import/token.json:ro" --entrypoint sh cpa-login \
    -c 'install -m 600 -o 10001 -g 10001 /import/token.json "/data/auth/$1"' sh "$name"
  say "imported $name"
}

# listen_addr: the Tailnet address clients use (what Docker publishes).
listen_addr() {
  "${compose[@]}" exec -T tollgate cat /run/tollgate/published 2>/dev/null | tr -d '\r\n'
}

cmd_setup() {
  need_tools
  ensure_host_ip
  say "1/4 secrets"
  ensure_secrets
  # Slack alerts (ADR-0019): only if configured; without, Slack stays off.
  if [ -n "${SLACK_BOT_TOKEN:-}" ] || [ -n "$(slack_sops_file)" ]; then
    cmd_slack
  fi
  say "2/4 build images"
  "${compose[@]}" --profile login build
  say "3/4 Claude account"
  local n
  n=$(tokens)
  if [ "$n" = 0 ]; then
    cmd_login
  else
    say "token present, skipping login"
  fi
  say "4/4 start and wait for health"
  "${compose[@]}" up -d --wait --wait-timeout 180
  local addr
  addr=$(listen_addr || true)
  [ -n "$addr" ] || die "tollgate did not report its address; see: just logs"
  say "tollgate runs on http://$addr (Tailnet only)"
  echo "  Frontend:      http://$addr/"
  echo "  Admin token:   $SECRETS/admin_token"
  echo "  Next step:     just client <name>"
}

cmd_up() {
  need_tools
  ensure_host_ip
  ensure_secrets
  "${compose[@]}" up -d --wait --wait-timeout 180
  echo "tollgate:      http://$(listen_addr || true)"
}

cmd_client() {
  local name=${1:-}
  [[ $name =~ ^[A-Za-z0-9._-]{1,64}$ ]] || die "usage: just client <name> (letters, digits, . _ -)"
  local addr body
  addr=$(listen_addr || true)
  [ -n "$addr" ] || die "tollgate is not running; run: just setup"
  body=$(printf '{"name":"%s"}' "$name")
  local resp
  # The admin token goes in via stdin, never as a process argument (ps).
  resp=$(curl -sS --fail-with-body -H @- \
    -H "Content-Type: application/json" -d "$body" "http://$addr/api/clients" \
    <<<"Authorization: Bearer $(<"$SECRETS/admin_token")") \
    || die "creating the client failed: $resp"
  local key
  key=$(printf '%s' "$resp" | sed -n 's/.*"key":"\([^"]*\)".*/\1/p')
  [ -n "$key" ] || die "unexpected answer from the admin API"
  say "client $name created; the key is shown only now"
  echo "export ANTHROPIC_BASE_URL=http://$addr"
  echo "export ANTHROPIC_AUTH_TOKEN=$key"
}

# cmd_speedtest sends the fixed speedtest request per model without and with
# the redact stage (#87). It uses subscription quota, so it asks first.
cmd_speedtest() {
  local yes=0 models=()
  for a in "$@"; do
    case $a in
      --yes) yes=1 ;;
      *) for m in ${a//,/ }; do models+=("$m"); done ;;
    esac
  done
  [ ${#models[@]} -ge 1 ] && [ ${#models[@]} -le 4 ] || die "usage: just speedtest models=\"<model>[,<model>…]\" (1 to 4 models)"
  local m list=""
  for m in "${models[@]}"; do
    [[ $m =~ ^[A-Za-z0-9_.:/\[\]-]{1,64}$ ]] || die "invalid model name: $m"
    list+="${list:+,}\"$m\""
  done
  local addr
  addr=$(listen_addr || true)
  [ -n "$addr" ] || die "tollgate is not running; run: just setup"
  if [ $yes -eq 0 ]; then
    local answer
    read -r -p "Der Test verbraucht Abo-Kontingent. Fortfahren? [j/N] " answer
    [[ $answer =~ ^[jJyY]$ ]] || die "abgebrochen"
  fi
  local resp
  # The admin token goes in via stdin, never as a process argument (ps).
  resp=$(curl -sS --fail-with-body --max-time 600 -H @- \
    -H "Content-Type: application/json" -d "{\"models\":[$list],\"confirm\":true}" "http://$addr/api/speedtest" \
    <<<"Authorization: Bearer $(cat "$SECRETS/admin_token")") || die "speedtest failed: $resp"
  printf '%-28s %-12s %10s %10s %8s %12s\n' MODELL VARIANTE TTFT DAUER T/S "ÜBLICH"
  printf '%s' "$resp" | sed 's/},{/}\n{/g' | while IFS= read -r r; do
    field() { printf '%s' "$r" | sed -n "s/.*\"$1\":\([^,}]*\).*/\1/p" | tr -d '"'; }
    ms() { [ -z "$1" ] || [ "$1" = null ] && { echo "–"; return; }; awk -v v="$1" 'BEGIN { printf "%.0f ms", v / 1000 }'; }
    local variant="ohne Redact"
    [ "$(field redact)" = true ] && variant="mit Redact"
    local err
    err=$(field error)
    if [ -n "$err" ]; then
      printf '%-28s %-12s %s\n' "$(field model)" "$variant" "$err"
    else
      printf '%-28s %-12s %10s %10s %8s %12s\n' "$(field model)" "$variant" "$(ms "$(field ttft_us)")" \
        "$(ms "$(field duration_us)")" "$(field output_tps)" "$(ms "$(field usual_duration_median_us)")"
    fi
  done
}

cmd_status() {
  need_tools
  host_ip_soft
  "${compose[@]}" ps
  local addr
  addr=$(listen_addr || true)
  echo "tollgate:      ${addr:-not running}"
  echo "Claude token:  $(tokens) file(s) in the auth dir"
}

case ${1:-setup} in
  setup) cmd_setup ;;
  up) cmd_up ;;
  down) need_tools; host_ip_soft; "${compose[@]}" down ;;
  logs) need_tools; host_ip_soft; "${compose[@]}" logs -f --tail 100 ;;
  login) cmd_login ;;
  import-token) cmd_import_token "${2:-}" ;;
  client) cmd_client "${2:-}" ;;
  status) cmd_status ;;
  speedtest) shift; cmd_speedtest "$@" ;;
  slack) cmd_slack ;;
  *) die "unknown command: $1" ;;
esac

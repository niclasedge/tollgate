#!/bin/sh
# Starts tollgate in the network namespace it shares with CLIProxyAPI
# (ADR-0013):
#   1. wait for an IPv4 address on the container interface,
#   2. copy the secrets to a private tmpfs with mode 0600 for the tollgate user,
#   3. write the bootstrap config and exec tollgate without root.
# Docker publishes the port only on the host's Tailnet IP; TOLLGATE_PUBLISHED
# names that address and tollgate refuses to start unless it is a Tailnet one.
# Secrets are never printed.
set -eu

iface=${TOLLGATE_LISTEN_IFACE:-eth0}
published=${TOLLGATE_PUBLISHED:-}
port=${TOLLGATE_PORT:-8443}
wait_s=${TOLLGATE_WAIT_SECONDS:-120}
run=/run/tollgate

ip=""
i=0
while [ "$i" -lt "$wait_s" ]; do
  ip=$(ip -4 -o addr show dev "$iface" 2>/dev/null | awk '{split($4, a, "/"); print a[1]; exit}')
  [ -n "$ip" ] && break
  i=$((i + 1))
  sleep 1
done
if [ -z "$ip" ]; then
  echo "tollgate-entrypoint: no IPv4 address on $iface after ${wait_s}s" >&2
  exit 1
fi

for s in tollgate_api_key admin_token; do
  if [ ! -s "/run/secrets/$s" ]; then
    echo "tollgate-entrypoint: secret $s is missing or empty (run: just setup)" >&2
    exit 1
  fi
done
install -m 600 -o tollgate -g tollgate /run/secrets/tollgate_api_key "$run/cpa.key"
install -m 600 -o tollgate -g tollgate /run/secrets/admin_token "$run/admin.token"
# Optional: the management key lets the dashboard list the CLIProxyAPI accounts.
mgmt_line=""
if [ -s /run/secrets/management_secret ]; then
  install -m 600 -o tollgate -g tollgate /run/secrets/management_secret "$run/cpa-mgmt.key"
  mgmt_line="  management_key_file: \"$run/cpa-mgmt.key\""
fi

pub_line=""
if [ -n "$published" ]; then
  case $published in
    *[!0-9A-Fa-f.:\[\]]*) echo "tollgate-entrypoint: TOLLGATE_PUBLISHED must be ip:port" >&2; exit 1 ;;
  esac
  pub_line="published: \"$published\""
fi

umask 077
cat > "$run/tollgate.yaml" <<EOF
listen: "$ip:$port"
${pub_line:+$pub_line
}upstream:
  url: "http://127.0.0.1:8317"
  api_key_file: "$run/cpa.key"
${mgmt_line:+$mgmt_line
}db:
  path: "/data/tollgate.db"
admin:
  token_file: "$run/admin.token"
crypto:
  key_file: "/data/crypto.key"
EOF
chown tollgate:tollgate "$run/tollgate.yaml"
# listen: container address for the healthcheck; published: address for clients.
printf '%s:%s\n' "$ip" "$port" > "$run/listen"
printf '%s\n' "${published:-$ip:$port}" > "$run/published"

set -- -config "$run/tollgate.yaml"
# Only for the smoke test without a Tailnet (deploy/compose.smoke.yml).
if [ "${TOLLGATE_DEV_ALLOW_ANY_LISTEN:-}" = 1 ]; then
  set -- "$@" --dev-allow-any-listen
fi
exec setpriv --reuid=tollgate --regid=tollgate --init-groups --no-new-privs \
  --bounding-set=-all --inh-caps=-all -- tollgate "$@"

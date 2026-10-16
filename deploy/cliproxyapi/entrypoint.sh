#!/bin/sh
# Entrypoint of the CLIProxyAPI image (ADR-0017).
#
# Usage: cpa-entrypoint [extra cli-proxy-api args, e.g. -claude-login -no-browser]
#
# Runs as root with NET_ADMIN only to restrict egress of the network namespace
# (tollgate shares it, ADR-0013):
#   1. OUTPUT policy DROP for IPv4 and IPv6; allowed are loopback, replies,
#      tcp/443 to Anthropic's published ranges, DNS from uid 0 (Docker's
#      embedded DNS forwarder) and IPv6 neighbour discovery.
#   2. Fail-closed: any failed rule stops the container before CLIProxyAPI runs.
#   3. Drop to the cpa user without capabilities, render the config and exec
#      CLIProxyAPI with -local-model (no remote model catalog).
# The nat table (Docker's DNS redirect) is never touched.
set -eu

# Anthropic's published inbound ranges (api.anthropic.com, platform.claude.com,
# claude.ai). Re-check on every CLIProxyAPI update (UPSTREAM.md).
ANTHROPIC_V4=160.79.104.0/23
ANTHROPIC_V6=2607:6bc0::/48

die() {
  echo "cpa-entrypoint: $*" >&2
  exit 1
}

run() {
  "$@" || die "egress rule failed, not starting: $*"
}

# pick <nft> <legacy>: the first backend that can list the filter table.
pick() {
  for c in "$1" "$2"; do
    if command -v "$c" >/dev/null 2>&1 && "$c" -w -S OUTPUT >/dev/null 2>&1; then
      echo "$c"
      return 0
    fi
  done
  return 1
}

# lock_down <cmd> <anthropic range>
lock_down() {
  ipt=$1
  run "$ipt" -w -P OUTPUT DROP
  run "$ipt" -w -F OUTPUT
  run "$ipt" -w -A OUTPUT -o lo -j ACCEPT
  run "$ipt" -w -A OUTPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
  run "$ipt" -w -A OUTPUT -p tcp -d "$2" --dport 443 -j ACCEPT
  run "$ipt" -w -A OUTPUT -p udp --dport 53 -m owner --uid-owner 0 -j ACCEPT
  run "$ipt" -w -A OUTPUT -p tcp --dport 53 -m owner --uid-owner 0 -j ACCEPT
}

# pick runs the -S probe; its call is part of the rule sequence.
ipt4=$(pick iptables iptables-legacy) || die "no usable iptables (needs NET_ADMIN), not starting"
lock_down "$ipt4" "$ANTHROPIC_V4"

ipt6=$(pick ip6tables ip6tables-legacy) || die "no usable ip6tables (needs NET_ADMIN), not starting"
lock_down "$ipt6" "$ANTHROPIC_V6"
for t in router-solicitation neighbour-solicitation neighbour-advertisement; do
  run "$ipt6" -w -A OUTPUT -p ipv6-icmp --icmpv6-type "$t" -j ACCEPT
done
echo "cpa-entrypoint: egress limited to Anthropic ($ipt4, $ipt6)" >&2

# shellcheck disable=SC2016 # expanded by the unprivileged shell
exec setpriv --reuid=cpa --regid=cpa --init-groups --no-new-privs \
  --bounding-set=-all --inh-caps=-all -- sh -c '
set -e
render-config.sh /etc/cliproxyapi/config.template.yaml /run/cpa/config.yaml /data/auth
exec cli-proxy-api -config /run/cpa/config.yaml -local-model "$@"' cpa-entrypoint "$@"

#!/bin/sh
# Renders deploy/cliproxyapi/config.yaml into a runnable config.
#
# Usage: render-config.sh <template> <out> <auth-dir>
# Secrets are read from files, never from arguments or the environment:
#   TOLLGATE_API_KEY_FILE   key tollgate uses towards CLIProxyAPI
#   MANAGEMENT_SECRET_FILE  management API key
set -eu

template=${1:?template path}
out=${2:?output path}
auth_dir=${3:?auth dir}
: "${TOLLGATE_API_KEY_FILE:?path to the tollgate API key file}"
: "${MANAGEMENT_SECRET_FILE:?path to the management secret file}"

api_key=$(tr -d '\r\n' < "$TOLLGATE_API_KEY_FILE")
mgmt=$(tr -d '\r\n' < "$MANAGEMENT_SECRET_FILE")

# Only URL-safe characters: keeps sed and YAML quoting trivial.
for v in "$api_key" "$mgmt"; do
  case $v in
    ''|*[!A-Za-z0-9_-]*) echo "render-config: secrets must be non-empty [A-Za-z0-9_-]" >&2; exit 1 ;;
  esac
  [ ${#v} -ge 32 ] || { echo "render-config: secrets must be at least 32 characters" >&2; exit 1; }
done
case $auth_dir in
  ""|*[!A-Za-z0-9_./-]*) echo "render-config: invalid auth dir" >&2; exit 1 ;;
esac

# Hard rule: only the token file of the active account lives in the auth dir.
set -- "$auth_dir"/*.json
if [ $# -gt 1 ]; then
  echo "render-config: auth dir holds more than one token file" >&2; exit 1
fi

umask 077
sed -e "s|@TOLLGATE_API_KEY@|$api_key|" \
    -e "s|@MANAGEMENT_SECRET@|$mgmt|" \
    -e "s|@AUTH_DIR@|$auth_dir|" "$template" > "$out"

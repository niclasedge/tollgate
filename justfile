# tollgate: setup and operations. Everything runs in containers (ADR-0013,
# docs/DEPLOY.md). Development checks stay in the Makefile (make test).

# Local agent tooling (ops-lab), not versioned; see docs/AGENT_SETUP.md.
# Local project recipes override generic imported development checks.
set allow-duplicate-recipes
import? 'tools/ops.just'

set shell := ["bash", "-euo", "pipefail", "-c"]

# List the recipes
default:
    @just --list

# Check prerequisites, create secrets, log in to Claude if needed, deploy
setup:
    deploy/setup.sh setup

# OAuth login with your Claude subscription (opens a URL to confirm)
login:
    deploy/setup.sh login

# Use an existing CLIProxyAPI token file instead of a new login
import-token file:
    deploy/setup.sh import-token "{{file}}"

# Create a client key and print the two environment variables for Claude Code
client name:
    deploy/setup.sh client "{{name}}"

# Slack alerts: write token and channel (env or TOLLGATE_SLACK_SOPS_FILE in .env)
slack-secret:
    deploy/setup.sh slack

# Speedtest per model, without and with the redact stage (uses subscription quota)
speedtest models *flags:
    deploy/setup.sh speedtest "{{models}}" {{flags}}

# Containers, Tailnet address and token state
status:
    deploy/setup.sh status

# Start the stack (after setup); refreshes the host Tailnet IP in .env
up:
    deploy/setup.sh up

# Stop the stack; volumes (token, database) stay
down:
    deploy/setup.sh down

# Follow the logs of all services
logs:
    deploy/setup.sh logs

# Build and test the stack without a Tailnet, then remove it again
smoke:
    deploy/smoke.sh

test *dateien:
    #!/usr/bin/env bash
    if [ -z "{{dateien}}" ]; then
        out=$(make test 2>&1) || { printf '%s\n' "$out"; exit 1; }; printf '%s\n' "$out" | tail -n 4
    else
        pakete=$(for f in {{dateien}}; do echo "./$(dirname "$f")"; done | sort -u)
        out=$(go test -race ${pakete:-./...} 2>&1) || { printf '%s\n' "$out"; exit 1; }; printf '%s\n' "$out" | tail -n 4
    fi


lint *dateien:
    #!/usr/bin/env bash
    if [ -z "{{dateien}}" ]; then
        out=$(make lint 2>&1) || { printf '%s\n' "$out"; exit 1; }
    else
        out=$(gofmt -l {{dateien}}) || exit 1
        [ -z "$out" ] || { printf '%s\n' "$out"; exit 1; }
        pakete=$(for f in {{dateien}}; do echo "./$(dirname "$f")"; done | sort -u)
        go vet $pakete || exit 1
    fi
    echo "lint ok"


cov:
    #!/usr/bin/env bash
    out=$(make web-test 2>&1) || { printf '%s\n' "$out"; exit 1; }; printf '%s\n' "$out" | tail -n 4
    mkdir -p coverage
    out=$(go test -race -coverprofile=coverage/cover.out ./... 2>&1) || { printf '%s\n' "$out"; exit 1; }; printf '%s\n' "$out" | tail -n 4
    go run github.com/jandelgado/gcov2lcov@latest -infile coverage/cover.out -outfile coverage/lcov.info

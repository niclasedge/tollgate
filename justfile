# tollgate: setup and operations. Everything runs in containers (ADR-0013,
# docs/DEPLOY.md). Development checks stay in the Makefile (make test).

# Local agent tooling (ops-lab), not versioned; see docs/AGENT_SETUP.md.
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

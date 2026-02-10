#!/bin/bash
# Plasma Shield module runner
# Called by homeboy module run with environment variables set

set -e

# Settings come from environment (prefixed with HOMEBOY_SETTING_)
API_URL="${HOMEBOY_SETTING_API_URL:-http://localhost:9000}"
API_KEY="${HOMEBOY_SETTING_API_KEY:-}"

# Build plasma-shield command
CMD="plasma-shield"

if [ -n "$API_URL" ]; then
  CMD="$CMD --api-url $API_URL"
fi

if [ -n "$API_KEY" ]; then
  CMD="$CMD --api-key $API_KEY"
fi

# Parse action from arguments
ACTION="${1:-status}"
shift || true

case "$ACTION" in
  status)
    $CMD status --json
    ;;
  agents)
    $CMD agent list --json
    ;;
  rules)
    $CMD rules list --json
    ;;
  logs)
    LIMIT="${HOMEBOY_INPUT_LIMIT:-50}"
    $CMD logs --limit "$LIMIT" --json
    ;;
  mode)
    AGENT="${HOMEBOY_INPUT_AGENT_ID:-}"
    MODE="${HOMEBOY_INPUT_MODE:-}"
    if [ -n "$AGENT" ] && [ -n "$MODE" ]; then
      $CMD agent mode "$AGENT" "$MODE" --json
    elif [ -n "$MODE" ]; then
      $CMD mode set "$MODE" --json
    else
      $CMD mode --json
    fi
    ;;
  *)
    echo "Unknown action: $ACTION" >&2
    exit 1
    ;;
esac

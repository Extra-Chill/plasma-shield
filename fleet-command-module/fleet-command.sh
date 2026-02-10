#!/bin/bash
# Fleet Command - Commodore-level fleet management
# Reads fleet registry, aggregates status, provides control

set -e

# Settings from environment
REGISTRY="${HOMEBOY_SETTING_FLEET_REGISTRY:-/var/lib/sweatpants/fleet-registry.yaml}"
SHIELD_API="${HOMEBOY_SETTING_SHIELD_API:-}"

# Inputs from environment  
VIEW="${HOMEBOY_INPUT_VIEW:-agents}"
AGENT_FILTER="${HOMEBOY_INPUT_AGENT:-}"
ACTION="${HOMEBOY_INPUT_ACTION:-none}"
LIMIT="${HOMEBOY_INPUT_LIMIT:-50}"

# Check dependencies
check_deps() {
  if ! command -v yq &> /dev/null; then
    echo '{"error": "yq not installed. Run: brew install yq"}' >&2
    exit 1
  fi
}

# Get agent list from registry
list_agents() {
  if [ ! -f "$REGISTRY" ]; then
    echo "[]"
    return
  fi
  
  yq -o=json '.agents | to_entries | map({
    "agent": .key,
    "ip": .value.ip,
    "description": .value.description,
    "webhook": .value.webhook_url,
    "status": "unknown",
    "mode": "unknown", 
    "shield": "not configured",
    "last_seen": "unknown"
  })' "$REGISTRY"
}

# Get shield status for agents (if shield API configured)
get_shield_status() {
  local agents="$1"
  
  if [ -z "$SHIELD_API" ]; then
    echo "$agents"
    return
  fi
  
  # Query shield API for agent modes
  local shield_data
  shield_data=$(curl -sf "$SHIELD_API/mode" 2>/dev/null || echo '{}')
  
  # Merge shield data with agent data
  echo "$agents" | jq --argjson shield "$shield_data" '
    map(. + {
      "mode": ($shield.agent_modes[.agent] // $shield.global_mode // "unknown"),
      "shield": (if $shield.global_mode then "active" else "not configured" end)
    })
  '
}

# Check agent health via ping
check_agent_health() {
  local agents="$1"
  
  echo "$agents" | jq 'map(. + {
    "status": "online"
  })'
}

# Apply agent filter
filter_agents() {
  local agents="$1"
  local filter="$2"
  
  if [ -z "$filter" ]; then
    echo "$agents"
  else
    echo "$agents" | jq --arg f "$filter" 'map(select(.agent | contains($f)))'
  fi
}

# Execute action on agent
execute_action() {
  local agent="$1"
  local action="$2"
  
  case "$action" in
    lockdown)
      if [ -n "$SHIELD_API" ]; then
        curl -sf -X PUT "$SHIELD_API/agent/$agent/mode" -d '{"mode":"lockdown"}' 2>/dev/null
      fi
      ;;
    audit)
      if [ -n "$SHIELD_API" ]; then
        curl -sf -X PUT "$SHIELD_API/agent/$agent/mode" -d '{"mode":"audit"}' 2>/dev/null
      fi
      ;;
    enforce)
      if [ -n "$SHIELD_API" ]; then
        curl -sf -X PUT "$SHIELD_API/agent/$agent/mode" -d '{"mode":"enforce"}' 2>/dev/null
      fi
      ;;
    ping)
      # Use sweatpants to ping agent
      if command -v sweatpants &> /dev/null; then
        local webhook
        webhook=$(yq ".agents.$agent.webhook_url" "$REGISTRY")
        sweatpants run fleet-ping --input agent="$agent" 2>/dev/null
      fi
      ;;
  esac
}

# Get traffic logs
get_traffic_logs() {
  if [ -z "$SHIELD_API" ]; then
    echo "[]"
    return
  fi
  
  curl -sf "$SHIELD_API/logs?limit=$LIMIT" 2>/dev/null || echo "[]"
}

# Get blocking rules
get_rules() {
  if [ -z "$SHIELD_API" ]; then
    echo "[]"
    return
  fi
  
  curl -sf "$SHIELD_API/rules" 2>/dev/null || echo "[]"
}

# Main logic
main() {
  check_deps
  
  case "$VIEW" in
    agents|shields)
      agents=$(list_agents)
      agents=$(get_shield_status "$agents")
      agents=$(check_agent_health "$agents")
      agents=$(filter_agents "$agents" "$AGENT_FILTER")
      
      # Execute action if specified
      if [ "$ACTION" != "none" ] && [ -n "$AGENT_FILTER" ]; then
        execute_action "$AGENT_FILTER" "$ACTION"
        # Refresh status after action
        agents=$(list_agents)
        agents=$(get_shield_status "$agents")
        agents=$(filter_agents "$agents" "$AGENT_FILTER")
      fi
      
      echo "$agents"
      ;;
    traffic)
      get_traffic_logs
      ;;
    rules)
      get_rules
      ;;
    *)
      echo "[]"
      ;;
  esac
}

main

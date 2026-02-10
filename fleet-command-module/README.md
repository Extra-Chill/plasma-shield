# Fleet Command — Homeboy Module

Commodore-level fleet management from Homeboy Desktop.

## Overview

Fleet Command provides a unified view of your entire AI agent armada:

- **Fleet Agents** — List all agents from your fleet registry
- **Shield Status** — View shield modes per agent
- **Traffic Logs** — Aggregated traffic across the fleet
- **Blocking Rules** — Manage fleet-wide rules

## Installation

```bash
homeboy module install https://github.com/Extra-Chill/plasma-shield/fleet-command-module
```

## Requirements

- `yq` for YAML parsing: `brew install yq`
- Fleet registry at `/var/lib/sweatpants/fleet-registry.yaml`
- (Optional) Plasma Shield router for shield status/control

## Fleet Registry Format

```yaml
agents:
  agent-name:
    webhook_url: https://example.com/agent-ping/
    token: "your-token"
    ip: 1.2.3.4
    description: "Agent description"

defaults:
  reply_channel: "discord-channel-id"
```

## Settings

| Setting | Description |
|---------|-------------|
| Fleet Registry Path | Path to fleet-registry.yaml |
| Shield Router API | Plasma Shield management API URL |
| Fleet Discord Channel | Default channel for fleet comms |

## Views

### Fleet Agents
Shows all registered agents with their status, IP, and shield configuration.

### Shield Status  
Focus on shield mode per agent (enforce/audit/lockdown).

### Traffic Logs
Aggregated traffic logs from the shield router.

### Blocking Rules
Current blocking rules across the fleet.

## Actions

- **Lockdown Selected** — Immediately block all traffic for selected agents
- **Ping Selected** — Send a health check to selected agents
- **Export CSV** — Export current view to CSV
- **Copy JSON** — Copy raw JSON data

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   Homeboy Desktop                       │
│                 (Fleet Command Module)                  │
└─────────────────────────┬───────────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          ▼               ▼               ▼
   Fleet Registry    Shield API     Agent Webhooks
   (local YAML)      (HTTP)         (Agent Ping)
```

Fleet Command aggregates data from multiple sources to give the Commodore a unified operational view.

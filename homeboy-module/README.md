# Plasma Shield Homeboy Module

Control Plasma Shield from Homeboy Desktop.

## Installation

```bash
# Install the CLI first
go install github.com/Extra-Chill/plasma-shield/cmd/plasma-shield@latest

# Install the module
homeboy module install https://github.com/Extra-Chill/plasma-shield/homeboy-module
```

## Features

- **Status** — View shield health and configuration
- **Agents** — List connected agents and their modes
- **Rules** — View blocking rules
- **Logs** — Stream traffic logs
- **Mode** — Change operating mode (enforce/audit/lockdown)

## Settings

Configure in Homeboy Desktop settings:

| Setting | Description |
|---------|-------------|
| Shield Router URL | API endpoint (default: `http://localhost:9000`) |
| API Key | Authentication key (optional) |

## Requirements

- Homeboy CLI and Desktop
- `plasma-shield` CLI installed
- Access to a Plasma Shield router

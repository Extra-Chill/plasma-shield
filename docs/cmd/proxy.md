# Plasma Shield Proxy

The core router service that inspects and filters all agent traffic. Run this on a dedicated VPS that agents cannot access directly.

## Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-proxy-addr` | `:8080` | Address for the proxy server |
| `-api-addr` | `127.0.0.1:9000` | Address for the management API and web UI (localhost only) |
| `-rules` | (none) | Path to rules YAML file |

## Environment Variables

None. All configuration is via command-line flags.

## Usage

```bash
# Basic startup (defaults)
plasma-shield-proxy

# Custom ports with rules file
plasma-shield-proxy -proxy-addr :8080 -api-addr 127.0.0.1:9000 -rules /etc/plasma-shield/rules.yaml

# Production example
plasma-shield-proxy \
  -proxy-addr :8080 \
  -api-addr 127.0.0.1:9000 \
  -rules /var/lib/plasma-shield/rules.yaml
```

## API Endpoints

The proxy exposes a management API on the `-api-addr` interface.

### Health & Status

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/health` | GET | None | Returns `OK` if running |
| `/` | GET | None | Web UI dashboard |

### Mode Management

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/mode` | GET | Get global mode and all agent modes |
| `/mode` | PUT/POST | Set global mode (`enforce`, `audit`, `lockdown`) |

**GET /mode response:**
```json
{
  "global_mode": "enforce",
  "agent_modes": {}
}
```

**PUT /mode request:**
```json
{"mode": "audit"}
```

### Per-Agent Mode

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/agent/{id}/mode` | GET | Get mode for specific agent |
| `/agent/{id}/mode` | PUT | Set mode for specific agent |
| `/agent/{id}/mode` | DELETE | Clear agent-specific mode (inherit global) |

### Traffic Logs

| Endpoint | Method | Query Params | Description |
|----------|--------|--------------|-------------|
| `/logs` | GET | `limit` (default: 50) | Get recent traffic logs |

Logs are stored in-memory (max 1000 entries) and returned most-recent-first.

### Rules

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/rules` | GET | Get rules file path and rule count |

### Exec Check (Agent Use)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/exec/check` | POST | Check if a command is allowed (for agents) |

### Fleet Management

| Endpoint | Method | Query Params | Description |
|----------|--------|--------------|-------------|
| `/fleet/mode` | GET | `tenant` (default: `default`) | Get fleet mode for tenant |
| `/fleet/mode` | PUT | `tenant` | Set fleet mode (`isolated`, `fleet`) |
| `/fleet/agents` | GET | `tenant` | List agents (respects fleet mode) |
| `/fleet/agents` | POST | `tenant` | Register an agent |
| `/fleet/can-communicate` | GET | `from`, `to` | Check if two agents can communicate |

**POST /fleet/agents request:**
```json
{"id": "agent-1", "name": "sarai"}
```

## Operating Modes

| Mode | Behavior |
|------|----------|
| `enforce` | Normal operation - block matching requests (default) |
| `audit` | Log everything but never block |
| `lockdown` | Block ALL outbound requests |

## Fleet Modes

| Mode | Behavior |
|------|----------|
| `isolated` | Agents cannot see each other |
| `fleet` | Agents can discover and communicate |

## Web Dashboard

Access via SSH tunnel:

```bash
ssh -L 9000:localhost:9000 root@<shield-ip>
# Open http://localhost:9000 in your browser
```

Features:
- Mode controls (enforce/audit/lockdown)
- Fleet mode toggle
- Traffic log viewer
- Agent management

## Graceful Shutdown

The proxy handles `SIGINT` and `SIGTERM` for graceful shutdown with a 10-second timeout.

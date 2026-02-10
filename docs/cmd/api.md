# Plasma Shield API Server

Standalone REST API server for human-only shield management. Run this on a dedicated VPS with the shield router.

## Command-Line Flags

| Flag | Default | Required | Description |
|------|---------|----------|-------------|
| `-addr` | `:8443` | No | API listen address |
| `-mgmt-token` | (none) | Yes | Management bearer token |
| `-agent-token` | (none) | Yes | Agent bearer token |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `PLASMA_MGMT_TOKEN` | Management bearer token (fallback for `-mgmt-token`) |
| `PLASMA_AGENT_TOKEN` | Agent bearer token (fallback for `-agent-token`) |

Tokens can be provided via flags or environment variables. At least one source is required for each token.

## Usage

```bash
# Minimal (tokens via environment)
export PLASMA_MGMT_TOKEN="your-management-token"
export PLASMA_AGENT_TOKEN="your-agent-token"
plasma-shield-api

# With flags
plasma-shield-api \
  -addr :8443 \
  -mgmt-token "your-management-token" \
  -agent-token "your-agent-token"

# Production example
plasma-shield-api \
  -addr :8443 \
  -mgmt-token "$PLASMA_MGMT_TOKEN" \
  -agent-token "$PLASMA_AGENT_TOKEN"
```

## Authentication

The API uses two separate bearer tokens for different access levels:

| Token | Purpose | Used By |
|-------|---------|---------|
| Management token | Full API access | CLI, operators |
| Agent token | Limited access (exec check only) | Agents |

Include in requests as:
```
Authorization: Bearer <token>
```

## API Endpoints

### No Auth Required

| Endpoint | Method | Description |
|----------|--------|-------------|
| `GET /health` | GET | Health check |

### Management Auth Required

| Endpoint | Method | Description |
|----------|--------|-------------|
| `GET /status` | GET | Shield status, version, uptime, statistics |
| `GET /agents` | GET | List all registered agents |
| `POST /agents/{id}/pause` | POST | Pause an agent |
| `POST /agents/{id}/kill` | POST | Kill an agent (emergency stop) |
| `POST /agents/{id}/resume` | POST | Resume a paused agent |
| `GET /rules` | GET | List all rules |
| `POST /rules` | POST | Create a new rule |
| `DELETE /rules/{id}` | DELETE | Delete a rule |
| `GET /logs` | GET | View traffic logs |

### Agent Auth Required

| Endpoint | Method | Description |
|----------|--------|-------------|
| `POST /exec/check` | POST | Check if a command is allowed |

## Endpoint Details

### GET /status

Returns shield status and statistics.

**Response:**
```json
{
  "status": "running",
  "version": "0.1.0",
  "uptime": "2h30m",
  "started_at": "2024-01-15T10:00:00Z",
  "agent_count": 3,
  "rule_count": 12,
  "requests_total": 1542,
  "blocked_total": 23
}
```

### GET /agents

List all registered agents.

**Response:**
```json
{
  "agents": [
    {
      "id": "agent-1",
      "name": "sarai",
      "ip": "178.156.229.129",
      "status": "active",
      "last_seen": "2024-01-15T12:30:00Z",
      "created_at": "2024-01-10T08:00:00Z"
    }
  ],
  "total": 1
}
```

### POST /agents/{id}/pause

Pause an agent (blocks all requests).

**Response:**
```json
{
  "id": "agent-1",
  "status": "paused",
  "message": "Agent paused"
}
```

### POST /agents/{id}/kill

Emergency stop an agent.

**Response:**
```json
{
  "id": "agent-1",
  "status": "killed",
  "message": "Agent killed"
}
```

### POST /agents/{id}/resume

Resume a paused or killed agent.

**Response:**
```json
{
  "id": "agent-1",
  "status": "active",
  "message": "Agent resumed"
}
```

### GET /rules

List all configured rules.

**Response:**
```json
{
  "rules": [
    {
      "id": "rule-abc123",
      "pattern": "rm -rf",
      "action": "block",
      "description": "Prevent recursive delete",
      "enabled": true,
      "created_at": "2024-01-10T08:00:00Z"
    }
  ],
  "total": 1
}
```

### POST /rules

Create a new rule.

**Request:**
```json
{
  "pattern": "rm -rf",
  "domain": "",
  "action": "block",
  "description": "Prevent recursive delete",
  "enabled": true
}
```

**Response:**
```json
{
  "rule": {
    "id": "rule-abc123",
    "pattern": "rm -rf",
    "action": "block",
    "description": "Prevent recursive delete",
    "enabled": true,
    "created_at": "2024-01-15T12:00:00Z"
  },
  "message": "Rule created"
}
```

### DELETE /rules/{id}

Delete a rule by ID.

**Response:**
```json
{
  "id": "rule-abc123",
  "message": "Rule deleted"
}
```

### GET /logs

View traffic logs.

**Query Parameters:**
- `limit` - Number of logs (default varies)
- `offset` - Pagination offset
- `agent_id` - Filter by agent
- `action` - Filter by action (`allowed`/`blocked`)
- `type` - Filter by type (`command`/`http`/`dns`)

**Response:**
```json
{
  "logs": [
    {
      "id": "log-xyz789",
      "timestamp": "2024-01-15T12:30:00Z",
      "agent_id": "agent-1",
      "type": "http",
      "request": "GET https://api.example.com/data",
      "action": "allowed",
      "rule_id": ""
    }
  ],
  "total": 150,
  "offset": 0,
  "limit": 100
}
```

### POST /exec/check

Check if a command is allowed (used by agents before executing).

**Request:**
```json
{
  "agent_id": "agent-1",
  "command": "ls -la /tmp"
}
```

**Response:**
```json
{
  "allowed": true,
  "reason": ""
}
```

Or if blocked:
```json
{
  "allowed": false,
  "reason": "matches rule: rule-abc123"
}
```

## Graceful Shutdown

The server handles `SIGINT` and `SIGTERM` for graceful shutdown with a 10-second timeout.

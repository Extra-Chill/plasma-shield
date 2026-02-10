# Plasma Shield CLI

Human-only management interface for the shield router. Install on your personal machine, not on agent VPSes.

## Command-Line Flags

### Global Flags

| Flag | Default | Env Var | Description |
|------|---------|---------|-------------|
| `--api-url` | `http://localhost:8443` | `PLASMA_API_URL` | Shield API URL |
| `--token` | (none) | `PLASMA_TOKEN` | Bearer auth token |
| `--json` | `false` | - | Output JSON for machine parsing |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `PLASMA_API_URL` | Shield API URL (fallback for `--api-url`) |
| `PLASMA_TOKEN` | Bearer auth token (fallback for `--token`) |

## Commands

### version

Show CLI version.

```bash
plasma-shield version
plasma-shield --version
plasma-shield -v
```

### status

Show shield connection status, version, uptime, and statistics.

```bash
plasma-shield status
plasma-shield --json status  # Machine-readable output
```

### mode

Set global operating mode.

```bash
plasma-shield mode <enforce|audit|lockdown>
```

| Mode | Behavior |
|------|----------|
| `enforce` | Normal operation - block matching requests (default) |
| `audit` | Log everything but never block |
| `lockdown` | Block ALL outbound requests |

**Examples:**
```bash
plasma-shield mode audit      # Enable audit mode
plasma-shield mode enforce    # Back to normal
plasma-shield mode lockdown   # Emergency stop
```

### agent

Manage agents in the fleet.

```bash
plasma-shield agent <list|pause|kill|resume> [agent-id]
```

| Action | Description |
|--------|-------------|
| `list` | List all registered agents |
| `pause` | Pause an agent (requires agent-id) |
| `kill` | Emergency stop an agent (requires agent-id) |
| `resume` | Resume a paused agent (requires agent-id) |

**Examples:**
```bash
plasma-shield agent list
plasma-shield agent pause sarai
plasma-shield agent kill sarai
plasma-shield agent resume sarai
```

### rules

Manage blocking rules.

```bash
plasma-shield rules <list|add|remove> [options]
```

#### rules list

List all configured rules.

```bash
plasma-shield rules list
```

#### rules add

Add a new rule.

| Flag | Default | Description |
|------|---------|-------------|
| `--pattern` | (none) | Command pattern to match |
| `--domain` | (none) | Domain to match |
| `--action` | `block` | Action: `block` or `allow` |
| `--desc` | (none) | Rule description |
| `--enabled` | `true` | Enable the rule |

Either `--pattern` or `--domain` is required.

```bash
plasma-shield rules add --pattern "rm -rf" --action block
plasma-shield rules add --domain "malicious.com" --action block --desc "Known bad domain"
plasma-shield rules add --pattern "curl" --action allow --desc "Allow curl commands"
```

#### rules remove

Remove a rule by ID.

```bash
plasma-shield rules remove <rule-id>
```

### logs

View traffic logs.

| Flag | Default | Description |
|------|---------|-------------|
| `--limit` | `100` | Number of logs to return |
| `--offset` | `0` | Offset for pagination |
| `--agent` | (none) | Filter by agent ID |
| `--action` | (none) | Filter by action: `allowed` or `blocked` |
| `--type` | (none) | Filter by type: `command`, `http`, or `dns` |

**Examples:**
```bash
plasma-shield logs
plasma-shield logs --limit 50 --agent sarai
plasma-shield logs --action blocked
plasma-shield logs --type http --limit 20
```

### auth

Authentication management.

```bash
plasma-shield auth <login|logout>
```

**Note:** Full authentication is not yet implemented. Use `PLASMA_TOKEN` environment variable or `--token` flag for now.

## Usage Examples

```bash
# Check shield status
plasma-shield status

# Enable audit mode for testing
plasma-shield mode audit

# List agents and their status
plasma-shield agent list

# Pause a misbehaving agent
plasma-shield agent pause sarai

# Emergency lockdown
plasma-shield mode lockdown

# View recent blocked requests
plasma-shield logs --action blocked --limit 20

# Add a rule to block dangerous patterns
plasma-shield rules add --pattern "rm -rf /" --action block --desc "Prevent recursive delete"

# Machine-readable output for scripting
plasma-shield --json agent list
plasma-shield --json logs --limit 10
```

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error (API error, invalid arguments, etc.) |

## JSON Output

Use `--json` flag for machine-parseable output. Errors are returned as:

```json
{
  "error": "error message",
  "code": 1
}
```

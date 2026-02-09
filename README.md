# Plasma Shield 🛡️

Network security router for AI agent fleets. Inspect, filter, and control all agent traffic from a human-only control plane.

## What It Does

Plasma Shield wraps your AI agents in a network-level security boundary that **agents cannot see, access, or modify**. All traffic flows through the shield, where it can be inspected, filtered, and blocked based on rules you define.

```
╔═══════════════════════════════════════════════════════════════════╗
║                    HUMAN CONTROL PLANE                            ║
║                   (Agents cannot reach this)                      ║
║                                                                   ║
║         CLI ─────▶ Shield Router ◀────── Dashboard               ║
║                          │                                        ║
╚══════════════════════════╬════════════════════════════════════════╝
                           │
          ═══════════ PLASMA SHIELD ═══════════
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
          Agent 1      Agent 2      Agent N
```

## Why?

AI agents with shell access can do a lot of damage — intentionally or not. Plasma Shield provides:

1. **Network isolation** — Agents can only reach the internet through the shield
2. **Command filtering** — Block destructive patterns (`rm -rf`, `curl | bash`, etc.)
3. **Domain blocking** — Prevent access to dangerous or unauthorized sites
4. **Audit logging** — See exactly what your agents are doing
5. **Kill switch** — Instantly freeze any agent from the control plane
6. **Human-only management** — Agents cannot disable their own safety net

## Threat Model

Plasma Shield protects against:

- **Honest mistakes** — Agent runs a destructive command by accident
- **Prompt injection** — Malicious input tricks agent into harmful actions
- **Exfiltration** — Agent tries to send sensitive data to unauthorized destinations
- **Lateral movement** — Compromised agent tries to reach other systems

It does NOT protect against:

- Physical access to agent hardware
- Compromise of the shield router itself
- Vulnerabilities in the agent's local OS (use proper hardening too)

## Components

| Component | Description |
|-----------|-------------|
| `plasma-shield-router` | The shield router — forwards traffic, enforces rules |
| `plasma-shield` | CLI for managing the shield from your terminal |
| `dashboard` | Homeboy module for monitoring and control (optional) |

## Quick Start

### 1. Deploy the Shield Router

```bash
# On a dedicated VPS (NOT on any agent machine)
curl -fsSL https://get.plasmashield.dev | bash

# Or with Docker
docker run -d -p 443:443 -p 8443:8443 ghcr.io/extra-chill/plasma-shield
```

### 2. Configure Agent Lockdown

On each agent VPS, run the lockdown script:

```bash
curl -fsSL https://get.plasmashield.dev/lockdown | bash -s -- \
  --shield-ip <ROUTER_IP> \
  --agent-token <TOKEN>
```

This configures iptables to force all traffic through the shield.

### 3. Manage via CLI

```bash
# Install the CLI on your personal machine
brew install extra-chill/tap/plasma-shield

# Connect to your shield
plasma-shield auth login

# View agents
plasma-shield agent list

# Add a blocking rule
plasma-shield rules add --pattern "rm -rf /" --action block

# Watch traffic in real-time
plasma-shield logs --tail

# Emergency stop an agent
plasma-shield agent kill <agent-id>
```

## Architecture

See [docs/architecture.md](docs/architecture.md) for the full design.

### Network Flow

```
Agent Process
     │
     ▼ (all outbound blocked except to shield)
iptables REDIRECT
     │
     ▼
Shield Router (inspect + filter)
     │
     ▼ (if allowed)
Internet
```

### Management Flow

```
Human (you)
     │
     ▼ (WireGuard / SSH / Tailscale)
Shield Management API
     │
     ▼
Rule changes, kill switches, logs
```

Agents cannot reach the management API. It's on a separate network interface that only accepts connections from authorized human operators.

## Default Rules

Plasma Shield ships with sensible defaults:

```yaml
# Block access to common bad neighborhoods  
- domain: "*.ru"
  action: block
  
- domain: "pastebin.com"
  action: block
```

Customize rules via CLI, API, or config file.

## Roadmap

- [x] Project scaffold
- [ ] Basic HTTP/HTTPS proxy
- [ ] Rule engine with pattern matching
- [ ] CLI for management
- [ ] Agent lockdown scripts (iptables)
- [ ] WireGuard management interface
- [ ] Dashboard UI
- [ ] Multi-tenant support (for Spawn)

## License

MIT — Use it, fork it, protect your agents.

## Credits

Inspired by the Protoss plasma shield from StarCraft. You must construct additional pylons.

Built by [Extra Chill](https://github.com/Extra-Chill) for the AI agent era.

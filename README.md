# Plasma Shield 🛡️

Network security for AI agent fleets. Nested isolation with information hiding — agents can't see each other, can't see the shield, can't see they're part of a network.

## The Problem

AI agents with shell access are powerful — and dangerous. They can:
- Make mistakes that destroy data
- Be tricked by prompt injection into harmful actions
- Exfiltrate sensitive information
- Probe and discover network topology

Traditional solutions (firewalls, VPCs, service meshes) weren't designed for AI. They control **access** but not **awareness**. An agent behind a firewall still knows the firewall exists.

## The Solution

Plasma Shield provides **network-level security with information hiding**:

1. **Invisible infrastructure** — Agents don't know they're behind a shield
2. **Tenant isolation** — Users can't see other users' agents
3. **Nested shields** — Shields within shields, each with independent rules
4. **Visibility ≠ Access** — Operators can monitor without being able to access
5. **Human-only control** — Agents cannot disable their own safety net

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Network (Operator View)                         │
│                  Operator can see, but NOT access                   │
│                                                                     │
│   ┌─────────────────────────┐     ┌─────────────────────────┐      │
│   │    User A's Shield      │     │    User B's Shield      │      │
│   │   ┌───────┐ ┌───────┐   │     │   ┌───────┐             │      │
│   │   │Agent 1│ │Agent 2│   │     │   │Agent 1│             │      │
│   │   └───────┘ └───────┘   │     │   └───────┘             │      │
│   │                         │     │                         │      │
│   └─────────────────────────┘     └─────────────────────────┘      │
│                                                                     │
│   ● Users don't know about each other                               │
│   ● Agents don't know they're behind a shield                       │
│   ● Operators can see topology but can't access servers             │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Use Cases

### Simple (Most Users)

One agent, one site. The shield is invisible safety infrastructure.

```
User ──▶ Agent ──▶ WordPress Site
              │
        [Shield wraps agent, user doesn't know or care]
```

### Fleet (Power Users)

Multiple agents, optional orchestration. User configures inter-agent permissions.

```
User ──▶ Fleet Command ──┬──▶ Agent A ──▶ Site A
                         ├──▶ Agent B ──▶ Site B
                         └──▶ Agent C ──▶ Site C
```

### Multi-Tenant (SaaS / Enterprise)

Many users, each with isolated fleets. Users invisible to each other.

```
[SaaS Infrastructure]
    │
    ├── User A's Fleet (isolated)
    ├── User B's Fleet (isolated)  
    ├── User C's Fleet (isolated)
    └── ... (thousands of users)
```

## Threat Model

Plasma Shield protects against:

| Threat | Protection |
|--------|------------|
| Honest mistakes | Domain/URL blocking, audit logging |
| Prompt injection | Shield can't be disabled by the agent |
| Data exfiltration | Block unauthorized destinations |
| Lateral movement | Tenant isolation, inter-agent rules |
| Network probing | Information hiding, no discovery endpoints |

It does NOT protect against:
- Physical access to hardware
- Compromise of the shield router itself
- Local exec commands (use OS-level hardening)

## Components

| Component | Description |
|-----------|-------------|
| `plasma-shield-gateway` | Full gateway: forward proxy (outbound) + reverse proxy (inbound) |
| `plasma-shield-router` | Forward proxy only (legacy, use gateway for production) |
| `plasma-shield` | CLI for human operators |
| Web Dashboard | Embedded UI at `localhost:9000` (via SSH tunnel) |
| `lockdown.sh` | Script to configure agent iptables |

## Traffic Flow

All traffic flows through the shield:

```
                    ┌─────────────────────┐
                    │   Plasma Shield     │
     Captain ──────►│      Gateway        │◄────── External Traffic
                    │                     │
                    │  :8080 (outbound)   │
                    │  :8443 (inbound)    │
                    └──────────┬──────────┘
                               │
           ┌───────────────────┼───────────────────┐
           │                   │                   │
           ▼                   ▼                   ▼
        [Agent 1]          [Agent 2]           [Agent N]
      (no public IP)     (no public IP)      (no public IP)
```

- **Outbound**: Agents use shield as HTTP_PROXY
- **Inbound**: External traffic routes through shield to agents
- **Agents have no public endpoints** - invisible to outside world

## Quick Start

### 1. Deploy the Shield Router

```bash
# On a dedicated VPS (NOT on any agent machine)
git clone https://github.com/Extra-Chill/plasma-shield
cd plasma-shield
make build
./plasma-shield-router --config config.yaml
```

### 2. Lock Down Each Agent

```bash
# On the agent VPS
curl -fsSL https://raw.githubusercontent.com/Extra-Chill/plasma-shield/main/provisioning/lockdown.sh | \
  bash -s -- --shield-ip <ROUTER_IP>
```

This configures iptables to force all traffic through the shield.

### 3. Access the Dashboard (Human Operator Only)

```bash
# SSH tunnel to access the web UI (API binds to localhost only)
# ONLY the human operator has SSH access to the shield
ssh -L 9000:localhost:9000 root@<ROUTER_IP>

# Open http://localhost:9000 in your browser
```

> ⚠️ **No agent — including the Commodore — should have SSH access to the shield.**
> If an agent can tunnel in, it can disable the shield. The shield must be external
> to and invisible from every agent it protects.

The dashboard provides:
- Real-time mode controls (enforce/audit/lockdown)
- Fleet mode toggle (isolated/fleet)
- Traffic log viewer
- Agent management
- Rule viewer

### 4. Manage via CLI (Human Operator Only)

```bash
# Install CLI on your personal machine (NOT on any agent server)
go install github.com/Extra-Chill/plasma-shield/cmd/plasma-shield@latest

# Configure connection
plasma-shield config set api-url https://shield.example.com:9000
plasma-shield config set api-key <your-key>

# View status
plasma-shield status

# List agents
plasma-shield agent list

# Add a blocking rule
plasma-shield rules add --domain "evil.com" --action block

# Emergency lockdown
plasma-shield agent mode <agent-id> lockdown
```

## Hierarchical Roles

| Role | Sees | Shield Access | Typical User |
|------|------|---------------|--------------|
| **Human Operator** | Everything | Full — SSH, admin, controls | The person (you) |
| **Commodore** | Fleet agents, network status (via anonymous push) | **None — doesn't know the shield exists** | Fleet orchestration AI |
| **Captain** | Their fleet only | None | End user's AI |
| **Crew** | Their assignment only | None | Site-level AI agent |

> **Key principle:** The shield is invisible to ALL agents, including the Commodore. The Commodore receives network status as anonymous system events — it cannot trace where they come from, and has no concept of a "shield" to circumvent. Only the human operator can access, configure, or disable the shield.

## Operating Modes

| Mode | Behavior |
|------|----------|
| **enforce** | Block matching requests (default) |
| **audit** | Log everything, block nothing (testing) |
| **lockdown** | Block ALL requests (emergency) |

```bash
# Set mode for specific agent
plasma-shield agent mode <agent-id> audit

# Set mode for entire fleet  
plasma-shield fleet mode <fleet-id> lockdown
```

## Architecture

See [docs/architecture.md](docs/architecture.md) for the full design.

Key principles:
- **External enforcement** — Shield runs outside agent environment
- **Nested isolation** — Shields within shields
- **Information hiding** — Agents don't know about the network
- **Fail-closed** — If shield is down, agents can't reach internet

## Development

```bash
# Build everything
make build

# Run tests
make test

# Run the proxy locally
make run-proxy

# Run the CLI
make run-cli
```

## Roadmap

- [x] Project scaffold
- [x] Architecture documentation
- [x] CLI implementation
- [x] Rule engine with pattern matching
- [x] Operating modes (enforce/audit/lockdown)
- [x] Proxy handler tests
- [x] API handler tests
- [x] HTTP/HTTPS proxy (core implementation)
- [x] Web dashboard (embedded Alpine.js UI)
- [ ] Agent lockdown scripts (iptables)
- [ ] SSH bastion service (temporary debug access)
- [ ] Access grant system (time-limited, logged, revocable)
- [ ] WireGuard management interface
- [ ] Multi-tenant support
- [ ] Distributed shield mesh

## License

MIT — Use it, fork it, protect your agents.

## Credits

Inspired by the Protoss plasma shield from StarCraft. You must construct additional pylons.

Built by [Extra Chill](https://github.com/Extra-Chill) for the AI agent era.

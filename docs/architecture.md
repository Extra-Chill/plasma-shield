# Plasma Shield Architecture

## Overview

Plasma Shield is a network-level security boundary for AI agent fleets. It operates on the principle that **agents cannot be trusted to enforce their own safety limits**.

## Core Principles

1. **External enforcement** — Security rules are enforced outside the agent's environment
2. **Network isolation** — Agents cannot bypass the shield at the network level
3. **Human-only control** — Management interfaces are inaccessible to agents
4. **Defense in depth** — Multiple layers of protection

## Components

### Shield Router (`proxy`)

The central component. Runs on a dedicated VPS that agents cannot access.

Responsibilities:
- Forward HTTP/HTTPS traffic from agents to the internet
- Inspect requests against the rule engine
- Block or allow based on rules
- Log all traffic for audit
- Expose management API (on separate network interface)

### Shield CLI (`plasma-shield`)

Human-only command-line interface. Installed on the operator's personal machine.

Communicates with the shield router over a secure channel (WireGuard, SSH tunnel, or Tailscale) that agents cannot reach.

### Rule Engine

Pattern-matching engine that evaluates:
- Domain names (via DNS/SNI inspection)
- URL patterns (via HTTP inspection)
- Request headers and bodies (optional deep inspection)

Rules are managed externally and pushed to the router. Agents cannot view or modify rules.

## Network Architecture

```
                    HUMAN CONTROL PLANE
                    (Agents cannot reach)
    ┌────────────────────────────────────────────┐
    │                                            │
    │   [Your Machine]                           │
    │       │                                    │
    │       │ WireGuard (10.100.0.0/24)         │
    │       │                                    │
    │       ▼                                    │
    │   [Shield Router]                          │
    │   ┌────────────────────────────────────┐   │
    │   │ wg0: 10.100.0.1                    │   │
    │   │   └─ :22 SSH                       │   │
    │   │   └─ :9000 Management API          │   │
    │   │                                    │   │
    │   │ eth0: PUBLIC_IP                    │   │
    │   │   └─ :443 Agent Proxy              │   │
    │   │   └─ :8443 Exec Inspection         │   │
    │   └────────────────────────────────────┘   │
    │                                            │
    └────────────────────────────────────────────┘
                         │
        ═══════════ PLASMA SHIELD ═══════════
                         │
                    AGENT PLANE
                    (Isolated network)
    ┌────────────────────────────────────────────┐
    │                                            │
    │   [Agent VPS 1]        [Agent VPS 2]      │
    │   ┌──────────────┐    ┌──────────────┐    │
    │   │ iptables:    │    │ iptables:    │    │
    │   │ OUTPUT DROP  │    │ OUTPUT DROP  │    │
    │   │ except →     │    │ except →     │    │
    │   │ SHIELD_IP    │    │ SHIELD_IP    │    │
    │   └──────────────┘    └──────────────┘    │
    │                                            │
    └────────────────────────────────────────────┘
```

## Traffic Flow

### Outbound Web Request

```
1. Agent runs: curl https://api.example.com/data

2. Agent's iptables sees outbound traffic
   → Only SHIELD_IP is allowed
   → Request goes to shield proxy

3. Shield proxy receives CONNECT request
   → Extracts domain: api.example.com
   → Checks against rules
   → If blocked: return 403
   → If allowed: forward to destination

4. Response flows back through proxy to agent
```

### Exec Commands

Exec commands run locally on the agent machine, not over the network. The shield cannot directly inspect them. Exec safety is enforced through:

1. **OS-level permissions** — Agent runs as limited user (no root, restricted paths)
2. **Container isolation** — Run OpenClaw in Docker with limited capabilities
3. **seccomp/AppArmor** — Kernel-level syscall restrictions (advanced)

The shield focuses on what it CAN control: network traffic. For exec, use defense in depth at the OS layer.

## Security Model

### What Agents CAN Do

- Make outbound HTTP/HTTPS requests (via proxy)
- Execute local commands (subject to OS permissions)
- Read/write files (subject to OS permissions)

### What Agents CANNOT Do

- Reach the internet without going through the shield
- Access the shield's management interface
- View or modify blocking rules
- Disable the iptables firewall (requires root, agent runs as limited user)
- Access other agents' traffic

### Trust Boundaries

```
┌─────────────────────────────────────────────────────────────┐
│ TRUSTED                                                     │
│                                                             │
│   • Shield router                                           │
│   • Your personal machine                                   │
│   • WireGuard keys                                          │
│   • Rule definitions                                        │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ UNTRUSTED                                                   │
│                                                             │
│   • AI agents                                               │
│   • Agent VPS environments                                  │
│   • User prompts (potential injection)                      │
│   • External APIs the agent calls                           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Operating Modes

The shield supports multiple operating modes, controlled via CLI or management API.

### Enforce Mode (default)

Normal operation. Requests matching block rules are rejected.

```
Agent → Shield → Rule Match? → Block (403) or Forward
```

### Audit Mode

Testing/debugging mode. All requests are logged but never blocked. The agent cannot distinguish audit mode from enforce mode — blocked requests still appear to fail from the agent's perspective (optional: can configure to pass through).

```
Agent → Shield → Rule Match? → Log + Forward (always)
```

Use cases:
- Initial deployment testing
- Debugging false positives
- Monitoring before enforcement

### Lockdown Mode

Emergency mode. ALL outbound requests are blocked. Use when an agent is compromised.

```
Agent → Shield → Block Everything (503)
```

### Per-Agent Overrides

Each agent can have its own mode override:

```bash
plasma-shield agent mode <agent-id> audit    # This agent in audit mode
plasma-shield agent mode <agent-id> enforce  # Back to normal
plasma-shield agent mode <agent-id> lockdown # Emergency stop
```

Global mode affects all agents without explicit overrides.

## Emergency Access

**Never store bypass keys on agent VPS.** If you need emergency access:

1. **Hetzner Console** — Out-of-band serial console via web UI. No network involved.
2. **Shield passthrough** — Temporarily set agent to audit mode from shield CLI.
3. **WireGuard direct** — If shield router has WireGuard to agent (Commodore-only).

The agent must never have a mechanism to bypass its own restrictions.

## Failure Modes

### Shield Router Down

If the shield router is unreachable:
- Agents cannot make ANY outbound requests
- Fail-closed by design
- Alerts sent to operator (if configured)

### Agent Compromise

If an agent is fully compromised:
- Attacker is still constrained by iptables
- Cannot reach arbitrary internet destinations
- Cannot disable firewall (no root)
- All traffic logged for forensics

### Management Key Compromise

If WireGuard key is compromised:
- Attacker could modify rules
- Mitigation: Rotate keys, use hardware tokens, audit logs

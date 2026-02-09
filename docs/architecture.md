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

### Shield CLI (`shield`)

Human-only command-line interface. Installed on the operator's personal machine.

Communicates with the shield router over a secure channel (WireGuard, SSH tunnel, or Tailscale) that agents cannot reach.

### Rule Engine

Pattern-matching engine that evaluates:
- Shell commands (via exec inspection)
- Domain names (via DNS/SNI inspection)
- URL patterns (via HTTP inspection)

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

### Exec Command (with OpenClaw integration)

```
1. OpenClaw receives tool call: exec("rm -rf /tmp/*")

2. OpenClaw's pre-exec hook calls shield:
   POST https://shield:8443/exec/check
   {"command": "rm -rf /tmp/*", "agent_token": "xxx"}

3. Shield checks command against rules
   → Pattern "rm -rf" matches
   → Returns: {"allowed": false, "rule": "block-rm-rf"}

4. OpenClaw blocks execution, returns error to model
```

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

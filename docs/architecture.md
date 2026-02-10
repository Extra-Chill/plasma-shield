# Plasma Shield Architecture

## Overview

Plasma Shield is a network-level security boundary for AI agent fleets. It operates on the principle that **agents cannot be trusted to enforce their own safety limits**.

The architecture is **nested** — shields within shields, like Gutenberg inner blocks. Each layer has independent rules, and trust does not cascade inward.

## Core Principles

1. **External enforcement** — Security rules are enforced outside the agent's environment
2. **Visibility ≠ Access** — Operators can see network topology without being able to access servers
3. **Nested isolation** — Shields can contain shields, each with independent rules
4. **Information hiding** — Agents don't know about other tenants or that they're part of a network
5. **Human-only control** — Management interfaces are inaccessible to agents

## Network Topology

Plasma Shield supports multiple deployment patterns, from simple single-agent setups to complex multi-tenant fleets.

### Simple (Most Users)

One user, one agent, one site. The shield is invisible infrastructure.

```
┌─────────────────────────────────────┐
│            User's Shield            │
│                                     │
│   [Agent] ──▶ [WordPress Site]      │
│                                     │
└─────────────────────────────────────┘
```

The user talks to their agent. The agent doesn't know it's behind a shield.

### Intermediate (Multi-Site User)

One user, multiple sites, each with an agent. No central orchestrator.

```
┌─────────────────────────────────────────────────────┐
│                   User's Shield                     │
│                                                     │
│   [Agent A] ──▶ [Site A]                            │
│                                      (isolated or   │
│   [Agent B] ──▶ [Site B]              connected)    │
│                                                     │
└─────────────────────────────────────────────────────┘
```

The user can configure whether agents can communicate with each other.

### Advanced (Fleet with Command)

One user, hierarchical fleet with orchestration layer.

```
┌──────────────────────────────────────────────────────────────┐
│                        User's Shield                         │
│                                                              │
│   [Fleet Command] ──┬──▶ [Agent A] ──▶ [Site A]             │
│         ▲           │                                        │
│         │           ├──▶ [Agent B] ──▶ [Site B]             │
│       User          │                                        │
│                     └──▶ [Agent C] ──▶ [Site C]             │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

User talks to Fleet Command. Fleet Command orchestrates the crew. The crew doesn't know about each other unless Fleet Command tells them.

### Multi-Tenant (SaaS)

Multiple users, each with their own isolated fleet. Users cannot see each other.

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Spawn Network (Operator View)                   │
│                  Operator can see, but NOT access                   │
│                                                                     │
│   ┌─────────────────────────┐     ┌─────────────────────────┐      │
│   │    User A's Shield      │     │    User B's Shield      │      │
│   │   ┌───────┐ ┌───────┐   │     │   ┌───────┐             │      │
│   │   │Site 1 │ │Site 2 │   │     │   │Site 1 │             │      │
│   │   └───────┘ └───────┘   │     │   └───────┘             │      │
│   │     (user configures    │     │                         │      │
│   │      inner isolation)   │     │                         │      │
│   └─────────────────────────┘     └─────────────────────────┘      │
│                                                                     │
│   Users don't know about each other                                 │
│   Users don't know they're part of Spawn                            │
│   Agents don't know Plasma Shield exists                            │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Hierarchical Roles

| Role | Visibility | Access | Responsibilities |
|------|------------|--------|------------------|
| **Commodore** | All tenants, all topology | None by default; must open specific shields | Infrastructure ops, security policy, tenant isolation |
| **Captain** | Their fleet only | Their fleet (within their shield) | Configure inter-agent permissions, manage their agents |
| **Crew** | Their assignment only | Their tools/APIs | Serve their Captain, unaware of fleet structure |

**Key insight:** Visibility does not grant access. The Commodore can see that User A has 3 agents, but cannot access those agents unless User A's shield is explicitly opened for a specific session.

## Nested Shields

Shields can be nested to arbitrary depth:

```
┌──────────────────────────────────────────────────────────────────┐
│  Outer Shield (Spawn Infrastructure)                             │
│  - Enforces tenant isolation                                     │
│  - Operator can see topology                                     │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  Middle Shield (User's Fleet)                              │  │
│  │  - User configures inter-agent rules                       │  │
│  │  - User can grant Fleet Command orchestration access       │  │
│  │                                                            │  │
│  │  ┌─────────────────────┐  ┌─────────────────────┐         │  │
│  │  │  Inner Shield (Opt) │  │  Inner Shield (Opt) │         │  │
│  │  │  Site A             │  │  Site B             │         │  │
│  │  │  - Extra isolation  │  │  - Extra isolation  │         │  │
│  │  │  - Premium feature  │  │  - Premium feature  │         │  │
│  │  └─────────────────────┘  └─────────────────────┘         │  │
│  │                                                            │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

**Trust does not cascade inward.** Opening the outer shield does not grant access to inner shields. Each layer must be explicitly opened.

## Information Hiding

This is the critical security property that differentiates Plasma Shield from traditional network isolation.

### What Agents Know

- Their own identity (name, purpose)
- Their Captain (the user who controls them)
- Allowed APIs and tools (per their configuration)

### What Agents Don't Know

- Other agents exist (even in the same fleet, unless explicitly told)
- Other users/tenants exist
- They're behind a Plasma Shield
- They're part of a larger network
- The network topology

### How It Works

**Inter-agent communication is masked.** When Fleet Command sends a message to Sarai, Sarai sees it as coming from the Captain (Chubes), not from Fleet Command. Agents never receive traffic that reveals other agents exist.

**Blocked requests appear as network failures.** When the shield blocks a request, the agent sees a timeout or connection refused — not a "Plasma Shield blocked this" error.

**No discovery endpoints.** There is no API an agent can call to enumerate the network.

## Components

### Shield Router (`plasma-shield-router`)

The central component. Runs on dedicated infrastructure that agents cannot access.

Responsibilities:
- Forward HTTP/HTTPS traffic from agents to the internet
- Inspect requests against the rule engine
- Enforce tenant isolation
- Mask inter-agent communication
- Log all traffic for audit
- Expose management API (on separate network interface)

### Shield CLI (`plasma-shield`)

Human-only command-line interface. Installed on the operator's machine.

Communicates with the shield router over a secure channel that agents cannot reach.

### Web Dashboard

Browser-based management interface embedded in the shield router. Designed for accessibility — manage your fleet from any device, including your phone at 3am during emergencies.

**Access:** `http://localhost:9000` via SSH tunnel

```bash
ssh -L 9000:localhost:9000 root@<shield-ip>
# Then open http://localhost:9000 in your browser
```

**Features:**
- Mode controls (enforce/audit/lockdown)
- Fleet mode toggle (isolated/fleet)
- Traffic log viewer with filtering
- Agent management
- Rule viewer

**Tech:** Alpine.js (~15KB), dark theme, mobile-responsive. No external dependencies — everything embedded in the Go binary.

**Security:** API binds to `127.0.0.1:9000` only. Never exposed publicly. SSH key = your identity.

### Rule Engine

Pattern-matching engine that evaluates:
- Domain names (via DNS/SNI inspection)
- URL patterns (via HTTP inspection)
- Request headers and bodies (optional deep inspection)

Rules exist at each shield level:
- Infrastructure rules (Commodore-defined, all tenants)
- Fleet rules (Captain-defined, their agents)
- Instance rules (per-agent overrides)

## Operating Modes

### Enforce Mode (default)

Normal operation. Requests matching block rules are rejected.

### Audit Mode

Testing mode. All requests are logged but never blocked. Useful for:
- Initial deployment testing
- Debugging false positives
- Monitoring before enforcement

### Lockdown Mode

Emergency mode. ALL outbound requests are blocked.

```bash
plasma-shield agent mode <agent-id> lockdown  # Freeze this agent
plasma-shield fleet mode <fleet-id> lockdown  # Freeze entire fleet
```

## Access Control

### Opening a Shield

To access a server protected by a shield, the Commodore must explicitly open it:

```bash
# Open User A's fleet shield for 1 hour
plasma-shield access grant --fleet user-a --duration 1h

# Open a specific agent within that fleet
plasma-shield access grant --agent user-a/site-1 --duration 30m
```

Access grants are:
- Time-limited (required)
- Logged (always)
- Revocable (immediately)
- Scoped (to specific shield level)

### Emergency Access

If the shield router is unreachable:
1. **Cloud Console** — Out-of-band serial console via hosting provider
2. **No bypass keys on agent** — The agent must never have a mechanism to bypass its own restrictions

## SSH Bastion Service (Planned)

For SaaS deployments, operators need a way to debug customer agents without storing SSH keys on the shield or giving agents SSH access to each other.

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   Shield Router                         │
│                                                         │
│   HTTP Proxy (:8080)     SSH Bastion (:2222)           │
│         │                       │                       │
│         └───── Admin Panel ─────┘                       │
│              (localhost:9000)                           │
└─────────────────────────────────────────────────────────┘
```

### Access Levels

| Role | Can Access | How |
|------|------------|-----|
| **Commodore** (SaaS admin) | Any tenant | Grant via admin panel |
| **Captain** (fleet owner) | Their own agents | Grant via their fleet panel |
| **Crew** (agents) | Nothing | No SSH capability |

### Flow

1. Operator opens admin panel (localhost via SSH tunnel to their machine)
2. Grants temporary access: `plasma-shield access grant --target <agent> --duration 30m`
3. Shield issues short-lived SSH certificate
4. Operator SSHs through bastion: `ssh -J bastion:2222 agent-hostname`
5. All commands logged, access auto-revokes when duration expires

### Key Principles

- **No standing access** — Every session requires explicit grant
- **No stored keys** — Bastion uses certificate-based auth, not key files
- **Time-boxed** — Access expires automatically
- **Fully logged** — Every command recorded for audit
- **Scoped** — Grant specifies exactly which agent(s)

### Not Implemented Yet

This is planned architecture. Current state requires direct SSH from operator machines with pre-distributed keys.

## Traffic Flow

### Outbound Web Request

```
1. Agent makes request to api.example.com

2. Agent's iptables forces traffic to shield router
   (only SHIELD_IP is reachable)

3. Shield router receives request
   → Checks tenant isolation (is this allowed?)
   → Checks domain/URL rules
   → Checks fleet-level rules
   → Checks instance-level rules
   → If any block: return failure
   → If all pass: forward to destination

4. Response flows back through shield to agent
```

### Inter-Agent Communication (masked)

```
1. Fleet Command sends task to Sarai via Agent Ping

2. Shield intercepts the message
   → Source: Fleet Command
   → Destination: Sarai

3. Shield rewrites the message
   → Source: Captain (Chubes)
   → Destination: Sarai

4. Sarai receives the message
   → Sees it as coming from her Captain
   → Has no knowledge of Fleet Command
```

## Exec Commands

Exec commands run locally on the agent machine, not over the network. The shield cannot directly inspect them.

Exec safety requires OS-level controls:
- **Limited user** — Agent runs as non-root
- **Docker isolation** — Container with `--cap-drop=ALL`
- **seccomp/AppArmor** — Kernel-level syscall restrictions
- **Restricted PATH** — Limited available commands

The shield focuses on network traffic. Exec is defense-in-depth at the OS layer.

## Failure Modes

### Shield Router Down

- Agents cannot make ANY outbound requests
- Fail-closed by design
- Alerts sent to operator

### Agent Compromise

- Attacker is still constrained by iptables
- Cannot reach arbitrary internet destinations
- Cannot disable firewall (no root)
- All traffic logged for forensics
- Shield can be set to lockdown mode

### Shield Compromise

If the shield router itself is compromised:
- All traffic potentially exposed
- Mitigation: Defense in depth, audit logs, rotation
- Shield router should be minimal attack surface (no unnecessary services)

## Scaling

### Single Shield Router

For small deployments (< 100 agents):
- One shield router handles all traffic
- Simpler management
- Single point of monitoring

### Distributed Shield Mesh

For large deployments (enterprise scale):
- Multiple shield routers in mesh
- Consistent policy distribution
- Regional routing for latency
- No single point of failure

Architecture for mesh is TBD based on real-world scaling needs.

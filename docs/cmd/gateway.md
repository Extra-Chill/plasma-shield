# plasma-shield-gateway

The full Plasma Shield gateway: forward proxy (outbound) + reverse proxy (inbound).

## Overview

The gateway runs both halves of the shield:

1. **Forward Proxy (outbound)** - Agents use this as HTTP_PROXY for all outbound traffic
2. **Reverse Proxy (inbound)** - External traffic to agents routes through this

This is the production deployment for Plasma Shield. Agents are invisible to the outside world; all traffic flows through the gateway.

## Usage

```bash
plasma-shield-gateway \
  --outbound :8080 \
  --inbound :8443 \
  --agents /etc/plasma-shield/fleet.yaml \
  --rules /etc/plasma-shield/rules.yaml
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--outbound` | `:8080` | Forward proxy port (outbound agent traffic) |
| `--inbound` | `:8443` | Reverse proxy port (inbound to agents) |
| `--agents` | `/etc/plasma-shield/agents.yaml` | Fleet configuration file |
| `--rules` | (none) | Rules file for filtering |

## Configuration

### Fleet Configuration (agents.yaml)

```yaml
tenants:
  - id: my-fleet
    mode: fleet  # or "isolated"
    agents:
      - id: agent-1
        name: "Agent One"
        ip: "10.0.0.1"
        webhook_url: "http://10.0.0.1:18789"
        tier: crew

tokens:
  - token: "${API_TOKEN}"  # from environment
    tenant_id: my-fleet
    name: "My API Token"
```

### Environment Variables

- `SHIELD_TOKEN_<TENANT>=<token>` - Register auth tokens (fallback if not in config)

## Traffic Flow

### Outbound (Agent → World)

```
Agent ---> [Forward Proxy :8080] ---> Internet
             |
             +-- Filter rules applied
             +-- Logging
```

Agents must be configured to use the shield as HTTP proxy:
```bash
export HTTP_PROXY=http://shield:8080
export HTTPS_PROXY=http://shield:8080
```

### Inbound (World → Agent)

```
Client ---> [Reverse Proxy :8443] ---> Agent
              |
              +-- Auth required (Bearer token)
              +-- Routes: /agent/{id}/...
              +-- Tenant isolation enforced
```

Clients access agents via:
```bash
curl -H "Authorization: Bearer $TOKEN" \
  https://shield:8443/agent/my-agent/hooks
```

## Security

- Agents have no public endpoints
- All traffic is logged
- Tenant isolation: tokens only grant access to owned agents
- Fleet mode: opt-in inter-agent communication

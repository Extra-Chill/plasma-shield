# Internal Packages

Documentation for Plasma Shield's internal packages.

## Package Overview

| Package | Purpose |
|---------|---------|
| [proxy](proxy.md) | HTTP/HTTPS forward proxy with traffic inspection |
| [api](api.md) | REST API for management and agent control |
| [rules](rules.md) | Rule engine with pattern matching |
| [mode](mode.md) | Operating modes (enforce/audit/lockdown) |
| [fleet](fleet.md) | Fleet mode for multi-tenant agent communication |
| [web](web.md) | Embedded web dashboard |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        AI Agent Traffic                          │
└─────────────────────────────────┬───────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                         proxy.Handler                            │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    proxy.Inspector                       │    │
│  │  ┌──────────────┐         ┌──────────────┐              │    │
│  │  │ rules.Engine │◄───────►│ mode.Manager │              │    │
│  │  └──────────────┘         └──────────────┘              │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
                                  │
                                  │ Allow/Block
                                  ▼
                          ┌───────────────┐
                          │   Upstream    │
                          └───────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                       Management API                             │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                      api.Server                          │    │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────────────┐   │    │
│  │  │  Handlers  │ │ Middleware │ │      Store         │   │    │
│  │  │ (status,   │ │ (auth,     │ │ (agents, rules,    │   │    │
│  │  │  agents,   │ │  logging)  │ │  logs, metrics)    │   │    │
│  │  │  rules)    │ │            │ │                    │   │    │
│  │  └────────────┘ └────────────┘ └────────────────────┘   │    │
│  └─────────────────────────────────────────────────────────┘    │
│                              │                                   │
│                              ▼                                   │
│  ┌────────────────────────────────────────────────────────┐     │
│  │                    web.Handler                          │     │
│  │              (Embedded Dashboard UI)                    │     │
│  └────────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      fleet.Manager                               │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │   Tenant A (Fleet Mode)      │   Tenant B (Isolated)      │  │
│  │   ┌─────┐ ┌─────┐ ┌─────┐   │   ┌─────┐ ┌─────┐          │  │
│  │   │ A1  │◄►│ A2  │◄►│ A3  │   │   │ B1  │ │ B2  │          │  │
│  │   └─────┘ └─────┘ └─────┘   │   └─────┘ └─────┘          │  │
│  │        (can communicate)     │    (isolated from each     │  │
│  │                              │        other)              │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## Package Dependencies

```
proxy
├── rules.Engine      Rule evaluation
├── mode.Manager      Mode-aware blocking
└── net/http          HTTP handling

api
├── net/http          HTTP server
└── encoding/json     JSON serialization

rules
├── regexp            Pattern matching
├── gopkg.in/yaml.v3  YAML parsing
└── sync              Thread safety

mode
└── sync              Thread safety

fleet
└── sync              Thread safety

web
├── embed             Static file embedding
└── net/http          File server
```

## Data Flow

### Request Processing

1. Agent sends HTTP/HTTPS request through proxy
2. `proxy.Handler` receives request
3. `proxy.Inspector` extracts domain and agent token
4. `rules.Engine` evaluates request against rules
5. `mode.Manager` determines if blocking should occur based on mode
6. Request is allowed through or blocked with 403

### Mode Decision Logic

```
┌────────────────┐
│ Rule Matched?  │
└───────┬────────┘
        │
   ┌────┴────┐
   │         │
  Yes        No
   │         │
   ▼         ▼
┌──────┐  ┌──────┐
│ Mode │  │ Mode │
└──┬───┘  └──┬───┘
   │         │
   ▼         ▼
┌──────────────────────────────────────────────┐
│ Enforce:  Block if matched, Allow if not     │
│ Audit:    Always Allow (log "would block")   │
│ Lockdown: Always Block                       │
└──────────────────────────────────────────────┘
```

## Common Integration Patterns

### Setting Up the Proxy

```go
// 1. Create rule engine and load rules
engine := rules.NewEngine()
engine.LoadRules("rules.yaml")

// 2. Create mode manager
modeManager := mode.NewManager()

// 3. Create inspector with both
inspector := proxy.NewInspector(engine, modeManager)

// 4. Create proxy handler
handler := proxy.NewHandler(inspector)

// 5. Start proxy
http.ListenAndServe(":8080", handler)
```

### Setting Up the API Server

```go
// 1. Configure server
cfg := api.ServerConfig{
    Addr:            ":9000",
    ManagementToken: "mgmt-token",
    AgentToken:      "agent-token",
    Version:         "1.0.0",
}

// 2. Create and start server
server := api.NewServer(cfg)
server.Start()
```

### Combined Proxy + API + Web

```go
// Rules and mode (shared)
engine := rules.NewEngine()
engine.LoadRules("rules.yaml")
modeManager := mode.NewManager()

// Proxy
inspector := proxy.NewInspector(engine, modeManager)
proxyHandler := proxy.NewHandler(inspector)

// API
apiCfg := api.ServerConfig{...}
apiServer := api.NewServer(apiCfg)

// Start services
go http.ListenAndServe(":8080", proxyHandler)      // Proxy
go http.ListenAndServe(":9000", apiServer)         // API
go http.ListenAndServe(":8000", web.Handler())     // Web UI
```

## Quick Reference

### Operating Modes

| Mode | Rule Match → Block? | No Match → Block? |
|------|---------------------|-------------------|
| Enforce | Yes | No |
| Audit | No | No |
| Lockdown | Yes | Yes |

### Agent States

| State | Traffic | Can Resume? |
|-------|---------|-------------|
| active | Flowing | N/A |
| paused | Blocked | Yes |
| killed | Blocked | No |

### Fleet Modes

| Mode | Agents See Each Other? | Can Communicate? |
|------|------------------------|------------------|
| Isolated | No | No |
| Fleet | Yes | Yes |

### Rule Pattern Syntax

| Type | Example | Matches |
|------|---------|---------|
| Command glob | `rm -rf *` | `rm -rf /tmp` |
| Domain exact | `evil.com` | `evil.com` only |
| Domain wildcard | `*.evil.com` | `sub.evil.com`, `evil.com` |
| Domain contains | `*xmr*` | `xmrpool.net`, `my.xmr.io` |

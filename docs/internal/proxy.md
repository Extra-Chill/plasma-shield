# internal/proxy

HTTP/HTTPS forward proxy with traffic inspection and request filtering.

## Overview

The proxy package implements a forward proxy that intercepts HTTP and HTTPS traffic from AI agents. It works with the `rules` and `mode` packages to inspect requests and enforce blocking policies.

## Types

### LogEntry

Represents a logged request for audit trails.

```go
type LogEntry struct {
    Timestamp  time.Time `json:"timestamp"`
    AgentToken string    `json:"agent_token,omitempty"`
    Domain     string    `json:"domain"`
    Method     string    `json:"method"`
    Action     string    `json:"action"` // "allow", "block", or "audit"
    Reason     string    `json:"reason,omitempty"`
}
```

### Handler

Main HTTP proxy handler. Implements `http.Handler`.

```go
type Handler struct {
    inspector *Inspector
    client    *http.Client
}
```

### Inspector

Traffic inspection component that coordinates with the rule engine and mode manager.

```go
type Inspector struct {
    engine      *rules.Engine
    modeManager *mode.Manager
}
```

### ExecCheckRequest / ExecCheckResponse

Request/response types for the `/exec/check` endpoint.

```go
type ExecCheckRequest struct {
    Command    string `json:"command"`
    AgentToken string `json:"agent_token,omitempty"`
}

type ExecCheckResponse struct {
    Allowed bool   `json:"allowed"`
    Reason  string `json:"reason,omitempty"`
}
```

## Functions

### NewHandler

Creates a new proxy handler with the given inspector.

```go
func NewHandler(inspector *Inspector) *Handler
```

The handler configures an HTTP client with:
- 30-second timeout
- No automatic redirect following (client handles redirects)

### NewInspector

Creates a new traffic inspector.

```go
func NewInspector(engine *rules.Engine, modeManager *mode.Manager) *Inspector
```

### NewExecCheckHandler

Creates a handler for command execution checks.

```go
func NewExecCheckHandler(inspector *Inspector) *ExecCheckHandler
```

## Inspector Methods

### ExtractHost

Extracts and normalizes the domain from a request.

```go
func (i *Inspector) ExtractHost(r *http.Request) string
```

- Uses `Host` header or URL host
- Strips port numbers
- Returns lowercase

### ExtractAgentToken

Extracts the agent token from the `X-Agent-Token` header.

```go
func (i *Inspector) ExtractAgentToken(r *http.Request) string
```

### CheckRequest

Mode-aware request checking. Returns whether to block, if a rule matched, and the reason.

```go
func (i *Inspector) CheckRequest(r *http.Request) (shouldBlock bool, ruleMatched bool, reason string)
```

Mode behavior:
- **Enforce**: Blocks if rule matched
- **Audit**: Never blocks, logs what would have been blocked
- **Lockdown**: Blocks everything

### CheckDomain

Direct domain check against the rule engine (not mode-aware).

```go
func (i *Inspector) CheckDomain(domain string) (bool, string)
```

### CheckCommand

Command pattern check against the rule engine.

```go
func (i *Inspector) CheckCommand(command string) (bool, string)
```

### Mode / IsLockdown

Query the current mode for an agent.

```go
func (i *Inspector) Mode(agentID string) mode.Mode
func (i *Inspector) IsLockdown(agentID string) bool
```

## Handler Behavior

### HTTP Requests (non-CONNECT)

1. Extract domain and agent token
2. Check request against rules (mode-aware)
3. Log the request (allow/block/audit)
4. If blocked: return 403 Forbidden
5. Forward request to upstream, stripping proxy headers
6. Copy response back to client

**Headers stripped before forwarding:**
- `Proxy-Connection`
- `X-Agent-Token` (prevents leaking to upstream)

### HTTPS CONNECT Tunnels

1. Extract domain and agent token
2. Check request against rules (mode-aware)
3. Log the request
4. If blocked: return 403 Forbidden
5. Connect to target host
6. Hijack client connection
7. Send "200 Connection Established"
8. Tunnel data bidirectionally

## Usage Example

```go
// Create dependencies
engine := rules.NewEngine()
engine.LoadRules("rules.yaml")
modeManager := mode.NewManager()

// Create inspector and handler
inspector := proxy.NewInspector(engine, modeManager)
handler := proxy.NewHandler(inspector)

// Start proxy server
http.ListenAndServe(":8080", handler)
```

## Testing Example

From `handler_test.go`:

```go
func testInspector(t *testing.T, rulesYAML string) *Inspector {
    engine := rules.NewEngine()
    if rulesYAML != "" {
        if err := engine.LoadRulesFromBytes([]byte(rulesYAML)); err != nil {
            t.Fatalf("failed to load rules: %v", err)
        }
    }
    modeManager := mode.NewManager()
    return NewInspector(engine, modeManager)
}

// Usage
inspector := testInspector(t, `
rules:
  - id: block-evil
    domain: "evil.com"
    action: block
    description: "Block evil domain"
    enabled: true
`)
handler := NewHandler(inspector)
```

## Package Dependencies

```
proxy
├── rules.Engine     - Rule matching
├── mode.Manager     - Mode-aware blocking decisions
└── net/http         - HTTP handling
```

## Log Format

Requests are logged as JSON to stdout:

```json
{"timestamp":"2024-01-15T10:30:00Z","agent_token":"agent-123","domain":"example.com","method":"GET","action":"allow","reason":""}
```

Actions:
- `allow` - Request permitted
- `block` - Request blocked
- `audit` - Would have been blocked, but in audit mode

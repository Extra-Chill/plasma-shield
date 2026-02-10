# internal/api

REST API for Plasma Shield management and agent control.

## Overview

The api package provides a complete REST API for managing agents, rules, and viewing logs. It includes authentication middleware, an in-memory store, and a configured HTTP server.

## Types

### Response Types

```go
// StatusResponse - GET /status
type StatusResponse struct {
    Status        string    `json:"status"`
    Version       string    `json:"version"`
    Uptime        string    `json:"uptime"`
    StartedAt     time.Time `json:"started_at"`
    AgentCount    int       `json:"agent_count"`
    RuleCount     int       `json:"rule_count"`
    RequestsTotal int64     `json:"requests_total"`
    BlockedTotal  int64     `json:"blocked_total"`
}

// Agent - registered agent
type Agent struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    IP        string    `json:"ip"`
    Status    string    `json:"status"` // "active", "paused", "killed"
    LastSeen  time.Time `json:"last_seen"`
    CreatedAt time.Time `json:"created_at"`
}

// Rule - filtering rule
type Rule struct {
    ID          string    `json:"id"`
    Pattern     string    `json:"pattern,omitempty"`  // Command pattern
    Domain      string    `json:"domain,omitempty"`   // Domain pattern
    Action      string    `json:"action"`             // "block" or "allow"
    Description string    `json:"description,omitempty"`
    Enabled     bool      `json:"enabled"`
    CreatedAt   time.Time `json:"created_at"`
}

// LogEntry - traffic log entry
type LogEntry struct {
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    AgentID   string    `json:"agent_id"`
    Type      string    `json:"type"` // "command", "http", "dns"
    Request   string    `json:"request"`
    Action    string    `json:"action"` // "allowed", "blocked"
    RuleID    string    `json:"rule_id,omitempty"`
}

// ErrorResponse - standard error
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    int    `json:"code"`
    Details string `json:"details,omitempty"`
}
```

### Configuration Types

```go
// ServerConfig - server configuration
type ServerConfig struct {
    Addr            string
    ManagementToken string
    AgentToken      string
    Version         string
}

// AuthConfig - authentication configuration
type AuthConfig struct {
    ManagementToken string
    AgentToken      string
}
```

## Server

### NewServer

Creates a configured API server with all routes and middleware.

```go
func NewServer(cfg ServerConfig) *Server
```

### Server Methods

```go
func (s *Server) Start() error                         // Start listening
func (s *Server) Shutdown(ctx context.Context) error   // Graceful shutdown
func (s *Server) RegisterAgent(id, name, ip string)    // Add agent (testing)
func (s *Server) Handlers() *Handlers                   // Access handlers
```

## API Endpoints

### Health Check (No Auth)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check, returns "ok" |

### Management Endpoints (Require Management Token)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/status` | GET | System status and metrics |
| `/agents` | GET | List all agents |
| `/agents/{id}/pause` | POST | Pause agent (block all traffic) |
| `/agents/{id}/kill` | POST | Kill agent (block + alert) |
| `/agents/{id}/resume` | POST | Resume paused agent |
| `/rules` | GET | List all rules |
| `/rules` | POST | Create a new rule |
| `/rules/{id}` | DELETE | Delete a rule |
| `/logs` | GET | Query traffic logs |

### Agent Endpoints (Require Agent Token)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/exec/check` | POST | Check if command is allowed |

## Middleware

### ManagementAuth

Bearer token authentication for management endpoints.

```go
func ManagementAuth(cfg *AuthConfig) func(http.Handler) http.Handler
```

### AgentAuth

Bearer token authentication for agent endpoints.

```go
func AgentAuth(cfg *AuthConfig) func(http.Handler) http.Handler
```

### JSONContentType

Sets `Content-Type: application/json` header.

```go
func JSONContentType(next http.Handler) http.Handler
```

## Store

In-memory state management (thread-safe).

```go
type Store struct {
    agents        map[string]*Agent
    rules         map[string]*Rule
    logs          []LogEntry
    startedAt     time.Time
    requestsTotal int64
    blockedTotal  int64
}

func NewStore() *Store
```

## Handlers

### Handlers Type

```go
type Handlers struct {
    store   *Store
    version string
}

func NewHandlers(store *Store, version string) *Handlers
```

### Handler Methods

```go
// Status
func (h *Handlers) StatusHandler(w http.ResponseWriter, r *http.Request)

// Agents
func (h *Handlers) ListAgentsHandler(w http.ResponseWriter, r *http.Request)
func (h *Handlers) PauseAgentHandler(w http.ResponseWriter, r *http.Request)
func (h *Handlers) KillAgentHandler(w http.ResponseWriter, r *http.Request)
func (h *Handlers) ResumeAgentHandler(w http.ResponseWriter, r *http.Request)

// Rules
func (h *Handlers) ListRulesHandler(w http.ResponseWriter, r *http.Request)
func (h *Handlers) CreateRuleHandler(w http.ResponseWriter, r *http.Request)
func (h *Handlers) DeleteRuleHandler(w http.ResponseWriter, r *http.Request)

// Logs
func (h *Handlers) ListLogsHandler(w http.ResponseWriter, r *http.Request)

// Exec Check
func (h *Handlers) ExecCheckHandler(w http.ResponseWriter, r *http.Request)
```

## Usage Example

```go
cfg := api.ServerConfig{
    Addr:            ":9000",
    ManagementToken: "mgmt-secret-token",
    AgentToken:      "agent-secret-token",
    Version:         "1.0.0",
}

server := api.NewServer(cfg)

// Optionally register agents
server.RegisterAgent("agent-1", "Production Agent", "10.0.0.1")

// Start server
if err := server.Start(); err != nil && err != http.ErrServerClosed {
    log.Fatal(err)
}
```

## API Examples

### Check Command Execution

```bash
curl -X POST http://localhost:9000/exec/check \
  -H "Authorization: Bearer agent-secret-token" \
  -H "Content-Type: application/json" \
  -d '{"command": "rm -rf /", "agent_id": "agent-1"}'
```

Response:
```json
{
  "allowed": false,
  "reason": "Dangerous command",
  "rule_id": "block-rm-rf"
}
```

### Create Rule

```bash
curl -X POST http://localhost:9000/rules \
  -H "Authorization: Bearer mgmt-secret-token" \
  -H "Content-Type: application/json" \
  -d '{
    "pattern": "rm -rf *",
    "action": "block",
    "description": "Block recursive delete",
    "enabled": true
  }'
```

### Query Logs

```bash
# All logs with pagination
curl "http://localhost:9000/logs?limit=50&offset=0" \
  -H "Authorization: Bearer mgmt-secret-token"

# Filter by agent
curl "http://localhost:9000/logs?agent_id=agent-1" \
  -H "Authorization: Bearer mgmt-secret-token"

# Filter by action
curl "http://localhost:9000/logs?action=blocked" \
  -H "Authorization: Bearer mgmt-secret-token"

# Filter by type
curl "http://localhost:9000/logs?type=command" \
  -H "Authorization: Bearer mgmt-secret-token"
```

### Pause/Kill/Resume Agent

```bash
# Pause - blocks all traffic
curl -X POST http://localhost:9000/agents/agent-1/pause \
  -H "Authorization: Bearer mgmt-secret-token"

# Kill - blocks traffic and sends alert
curl -X POST http://localhost:9000/agents/agent-1/kill \
  -H "Authorization: Bearer mgmt-secret-token"

# Resume - only works for paused agents (not killed)
curl -X POST http://localhost:9000/agents/agent-1/resume \
  -H "Authorization: Bearer mgmt-secret-token"
```

## Agent States

| State | Description | Can Resume? |
|-------|-------------|-------------|
| `active` | Normal operation | N/A |
| `paused` | All traffic blocked | Yes |
| `killed` | Traffic blocked + alert sent | No (use restore) |

## Log Retention

The store automatically trims logs to keep only the most recent 10,000 entries.

## Testing Example

From `handlers_test.go`:

```go
func TestExecCheckHandler(t *testing.T) {
    store := NewStore()
    handlers := NewHandlers(store, "1.0.0")
    handlers.RegisterAgent("agent-1", "Test Agent", "192.168.1.1")

    store.mu.Lock()
    store.rules["rule-1"] = &Rule{
        ID:          "rule-1",
        Pattern:     "rm -rf",
        Action:      "block",
        Description: "Dangerous command",
        Enabled:     true,
    }
    store.mu.Unlock()

    body := ExecCheckRequest{
        Command: "rm -rf /",
        AgentID: "agent-1",
    }
    bodyBytes, _ := json.Marshal(body)

    req := httptest.NewRequest(http.MethodPost, "/exec/check", bytes.NewReader(bodyBytes))
    rec := httptest.NewRecorder()

    handlers.ExecCheckHandler(rec, req)

    // Should be blocked
    var resp ExecCheckResponse
    json.NewDecoder(rec.Body).Decode(&resp)
    
    if resp.Allowed {
        t.Error("expected command to be blocked")
    }
}
```

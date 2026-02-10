# internal/fleet

Fleet management for multi-tenant agent communication.

## Overview

The fleet package manages tenant fleets and controls whether agents can discover and communicate with each other. In **isolated mode**, agents are unaware of other agents. In **fleet mode**, agents can see and communicate with other agents in their tenant.

## Mode Types

```go
type Mode string

const (
    Isolated Mode = "isolated"  // Agents can't see or communicate with each other
    Fleet    Mode = "fleet"     // Agents can discover and communicate
)
```

## Types

### Agent

Represents an agent in a fleet.

```go
type Agent struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    IP          string `json:"ip,omitempty"`
    WebhookURL  string `json:"webhook_url,omitempty"`
    Description string `json:"description,omitempty"`
}
```

### Tenant

A customer/user with their fleet configuration.

```go
type Tenant struct {
    ID     string           `json:"id"`
    Mode   Mode             `json:"mode"`
    Agents map[string]Agent `json:"agents"`
}
```

### Manager

Fleet manager for multi-tenant agent coordination.

```go
type Manager struct {
    tenants       map[string]*Tenant
    agentToTenant map[string]string  // Quick agent → tenant lookup
}
```

## Functions

### NewManager

Creates a new fleet manager.

```go
func NewManager() *Manager
```

### Tenant Management

```go
// Create tenant with default isolated mode
func (m *Manager) CreateTenant(tenantID string) *Tenant

// Get tenant by ID
func (m *Manager) GetTenant(tenantID string) *Tenant

// Get tenant ID for an agent
func (m *Manager) GetTenantForAgent(agentID string) string

// List all tenant IDs
func (m *Manager) AllTenants() []string

// Get tenant info (id, mode, agent_count)
func (m *Manager) GetTenantInfo(tenantID string) map[string]interface{}
```

### Mode Management

```go
// Set fleet mode for tenant
func (m *Manager) SetMode(tenantID string, mode Mode)

// Get fleet mode (defaults to Isolated)
func (m *Manager) GetMode(tenantID string) Mode
```

### Agent Management

```go
// Add agent to tenant's fleet
func (m *Manager) AddAgent(tenantID string, agent Agent)

// Remove agent from fleet
func (m *Manager) RemoveAgent(tenantID, agentID string)

// Get all visible agents in tenant's fleet
// Returns empty in Isolated mode
func (m *Manager) GetAgents(tenantID string) []Agent

// Get agents visible to a specific agent (excludes self)
// Returns empty in Isolated mode
func (m *Manager) GetAgentsForAgent(agentID string) []Agent
```

### Communication Control

```go
// Check if two agents can communicate
// True only if same tenant AND fleet mode
func (m *Manager) CanCommunicate(fromAgentID, toAgentID string) bool
```

## Mode Comparison

| Feature | Isolated | Fleet |
|---------|----------|-------|
| Agent knows about others | ❌ | ✅ |
| GetAgents() returns list | ❌ (empty) | ✅ |
| CanCommunicate() returns true | ❌ | ✅ |
| Cross-tenant communication | ❌ | ❌ |

## Usage Examples

### Basic Setup

```go
fm := fleet.NewManager()

// Create tenant
fm.CreateTenant("acme-corp")

// Add agents
fm.AddAgent("acme-corp", fleet.Agent{
    ID:   "agent-1",
    Name: "Production Agent",
    IP:   "10.0.0.1",
})

fm.AddAgent("acme-corp", fleet.Agent{
    ID:   "agent-2",
    Name: "Development Agent",
    IP:   "10.0.0.2",
})

// Default mode is Isolated - agents can't see each other
agents := fm.GetAgents("acme-corp")
// agents is empty []
```

### Enable Fleet Mode

```go
fm := fleet.NewManager()
fm.CreateTenant("acme-corp")

fm.AddAgent("acme-corp", fleet.Agent{ID: "agent-1", Name: "Agent 1"})
fm.AddAgent("acme-corp", fleet.Agent{ID: "agent-2", Name: "Agent 2"})
fm.AddAgent("acme-corp", fleet.Agent{ID: "agent-3", Name: "Agent 3"})

// Enable fleet mode
fm.SetMode("acme-corp", fleet.Fleet)

// Now agents can see each other
agents := fm.GetAgents("acme-corp")
// agents contains all 3 agents

// Get agents visible to a specific agent (excludes self)
visible := fm.GetAgentsForAgent("agent-1")
// visible contains agent-2 and agent-3
```

### Inter-Agent Communication Check

```go
fm := fleet.NewManager()

// Set up two tenants
fm.AddAgent("tenant-1", fleet.Agent{ID: "a1"})
fm.AddAgent("tenant-1", fleet.Agent{ID: "a2"})
fm.AddAgent("tenant-2", fleet.Agent{ID: "b1"})

fm.SetMode("tenant-1", fleet.Fleet)
fm.SetMode("tenant-2", fleet.Fleet)

// Same tenant, fleet mode - can communicate
fm.CanCommunicate("a1", "a2") // true

// Different tenants - cannot communicate
fm.CanCommunicate("a1", "b1") // false

// Same tenant but isolated mode - cannot communicate
fm.SetMode("tenant-1", fleet.Isolated)
fm.CanCommunicate("a1", "a2") // false
```

### Agent Discovery Pattern

```go
// Agent queries for peer agents
func discoverPeers(fm *fleet.Manager, agentID string) []fleet.Agent {
    return fm.GetAgentsForAgent(agentID)
}

// In fleet mode, returns list of peers
// In isolated mode, returns empty list
peers := discoverPeers(fm, "agent-1")

for _, peer := range peers {
    // Can reach peer via webhook
    if peer.WebhookURL != "" {
        // Send message to peer
    }
}
```

## Multi-Tenant Isolation

The fleet package ensures strict tenant isolation:

1. Agents can only see other agents in their own tenant
2. `CanCommunicate()` returns false for cross-tenant requests
3. Each tenant has independent mode settings

```go
fm := fleet.NewManager()

// Company A - fleet mode (collaborative)
fm.AddAgent("company-a", fleet.Agent{ID: "a1"})
fm.AddAgent("company-a", fleet.Agent{ID: "a2"})
fm.SetMode("company-a", fleet.Fleet)

// Company B - isolated mode (independent)
fm.AddAgent("company-b", fleet.Agent{ID: "b1"})
fm.AddAgent("company-b", fleet.Agent{ID: "b2"})
// mode defaults to Isolated

// Company A agents can see each other
fm.GetAgentsForAgent("a1") // returns [a2]

// Company B agents cannot see each other
fm.GetAgentsForAgent("b1") // returns []

// Cross-company communication always blocked
fm.CanCommunicate("a1", "b1") // false
```

## Thread Safety

The Manager is thread-safe:
- All operations use read/write locks
- Safe for concurrent tenant and agent management

## Testing Examples

From `fleet_test.go`:

```go
func TestGetAgentsFleetMode(t *testing.T) {
    m := NewManager()
    m.AddAgent("tenant1", Agent{ID: "agent-1", Name: "Agent 1"})
    m.AddAgent("tenant1", Agent{ID: "agent-2", Name: "Agent 2"})
    m.SetMode("tenant1", Fleet)
    
    agents := m.GetAgents("tenant1")
    if len(agents) != 2 {
        t.Errorf("expected 2 agents in fleet mode, got %d", len(agents))
    }
}

func TestCanCommunicateDifferentTenants(t *testing.T) {
    m := NewManager()
    m.AddAgent("tenant1", Agent{ID: "agent-1"})
    m.AddAgent("tenant2", Agent{ID: "agent-2"})
    m.SetMode("tenant1", Fleet)
    m.SetMode("tenant2", Fleet)
    
    // Even in fleet mode, cross-tenant is blocked
    if m.CanCommunicate("agent-1", "agent-2") {
        t.Error("agents from different tenants should not communicate")
    }
}
```

## Common Patterns

### Isolated Development, Fleet Production

```go
fm := fleet.NewManager()

// Development tenant - each agent works independently
fm.CreateTenant("dev")
fm.SetMode("dev", fleet.Isolated)

// Production tenant - agents collaborate
fm.CreateTenant("prod")
fm.SetMode("prod", fleet.Fleet)
```

### Dynamic Mode Switching

```go
// Start isolated for initial deployment
fm.SetMode("tenant-1", fleet.Isolated)

// Enable fleet mode when ready for agent collaboration
fm.SetMode("tenant-1", fleet.Fleet)

// Temporarily isolate for maintenance
fm.SetMode("tenant-1", fleet.Isolated)
```

// Package fleet manages tenant fleets and inter-agent communication modes.
package fleet

import (
	"sync"
)

// Mode represents the fleet communication mode.
type Mode string

const (
	// Isolated means agents don't know about each other and can't communicate.
	Isolated Mode = "isolated"
	// Fleet means agents know about each other and can communicate.
	Fleet Mode = "fleet"
)

// Agent represents an agent in a fleet.
type Agent struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IP          string `json:"ip,omitempty"`
	WebhookURL  string `json:"webhook_url,omitempty"`
	Tier        string `json:"tier,omitempty"` // "commodore", "captain", "crew"
	Description string `json:"description,omitempty"`
}

// Tenant represents a customer/user with their fleet configuration.
type Tenant struct {
	ID     string           `json:"id"`
	Mode   Mode             `json:"mode"`
	Agents map[string]Agent `json:"agents"`
}

// Manager handles fleet configuration and inter-agent communication rules.
type Manager struct {
	mu      sync.RWMutex
	tenants map[string]*Tenant
	// agentToTenant maps agent IDs to tenant IDs for quick lookup
	agentToTenant map[string]string
	// ipToAgent maps IP addresses to agent info for fast validation
	ipToAgent map[string]*Agent
}

// NewManager creates a new fleet manager.
func NewManager() *Manager {
	return &Manager{
		tenants:       make(map[string]*Tenant),
		agentToTenant: make(map[string]string),
		ipToAgent:     make(map[string]*Agent),
	}
}

// ValidateAgentIP checks if an IP belongs to a registered agent.
// Returns agent ID and tier if valid, empty strings if not.
// Implements the proxy.AgentRegistry interface.
func (m *Manager) ValidateAgentIP(ip string) (agentID string, tier string, valid bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, ok := m.ipToAgent[ip]
	if !ok {
		return "", "", false
	}

	tier = agent.Tier
	if tier == "" {
		tier = "crew" // Default tier
	}

	return agent.ID, tier, true
}

// GetAgentByIP returns the agent with the given IP.
func (m *Manager) GetAgentByIP(ip string) *Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ipToAgent[ip]
}

// CreateTenant creates a new tenant with default isolated mode.
func (m *Manager) CreateTenant(tenantID string) *Tenant {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenant := &Tenant{
		ID:     tenantID,
		Mode:   Isolated,
		Agents: make(map[string]Agent),
	}
	m.tenants[tenantID] = tenant
	return tenant
}

// GetTenant returns a tenant by ID.
func (m *Manager) GetTenant(tenantID string) *Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tenants[tenantID]
}

// GetTenantForAgent returns the tenant ID for an agent.
func (m *Manager) GetTenantForAgent(agentID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agentToTenant[agentID]
}

// SetMode sets the fleet mode for a tenant.
func (m *Manager) SetMode(tenantID string, mode Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenant, exists := m.tenants[tenantID]
	if !exists {
		tenant = &Tenant{
			ID:     tenantID,
			Mode:   mode,
			Agents: make(map[string]Agent),
		}
		m.tenants[tenantID] = tenant
	} else {
		tenant.Mode = mode
	}
}

// GetMode returns the fleet mode for a tenant.
func (m *Manager) GetMode(tenantID string) Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenant, exists := m.tenants[tenantID]
	if !exists {
		return Isolated // Default
	}
	return tenant.Mode
}

// AddAgent adds an agent to a tenant's fleet.
func (m *Manager) AddAgent(tenantID string, agent Agent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenant, exists := m.tenants[tenantID]
	if !exists {
		tenant = &Tenant{
			ID:     tenantID,
			Mode:   Isolated,
			Agents: make(map[string]Agent),
		}
		m.tenants[tenantID] = tenant
	}

	tenant.Agents[agent.ID] = agent
	m.agentToTenant[agent.ID] = tenantID

	// Add IP lookup if agent has IP
	if agent.IP != "" {
		agentCopy := agent // Store copy so pointer remains valid
		m.ipToAgent[agent.IP] = &agentCopy
	}
}

// RemoveAgent removes an agent from a tenant's fleet.
func (m *Manager) RemoveAgent(tenantID, agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenant, exists := m.tenants[tenantID]
	if !exists {
		return
	}

	// Remove IP lookup
	if agent, ok := tenant.Agents[agentID]; ok && agent.IP != "" {
		delete(m.ipToAgent, agent.IP)
	}

	delete(tenant.Agents, agentID)
	delete(m.agentToTenant, agentID)
}

// GetAgents returns all agents in a tenant's fleet.
// In isolated mode, this returns an empty list (agents don't know about each other).
func (m *Manager) GetAgents(tenantID string) []Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenant, exists := m.tenants[tenantID]
	if !exists {
		return nil
	}

	// In isolated mode, agents don't know about each other
	if tenant.Mode == Isolated {
		return nil
	}

	agents := make([]Agent, 0, len(tenant.Agents))
	for _, agent := range tenant.Agents {
		agents = append(agents, agent)
	}
	return agents
}

// GetAgentsForAgent returns the agents visible to a specific agent.
// In isolated mode, returns empty (agent doesn't know about others).
// In fleet mode, returns all other agents in the fleet.
func (m *Manager) GetAgentsForAgent(agentID string) []Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenantID, exists := m.agentToTenant[agentID]
	if !exists {
		return nil
	}

	tenant := m.tenants[tenantID]
	if tenant == nil || tenant.Mode == Isolated {
		return nil
	}

	// Return all agents except self
	agents := make([]Agent, 0, len(tenant.Agents)-1)
	for _, agent := range tenant.Agents {
		if agent.ID != agentID {
			agents = append(agents, agent)
		}
	}
	return agents
}

// CanCommunicate checks if two agents can communicate with each other.
// Returns true only if both agents are in the same tenant AND fleet mode is enabled.
func (m *Manager) CanCommunicate(fromAgentID, toAgentID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fromTenant := m.agentToTenant[fromAgentID]
	toTenant := m.agentToTenant[toAgentID]

	// Must be in the same tenant
	if fromTenant == "" || fromTenant != toTenant {
		return false
	}

	tenant := m.tenants[fromTenant]
	if tenant == nil {
		return false
	}

	// Must be in fleet mode
	return tenant.Mode == Fleet
}

// AllTenants returns all tenant IDs.
func (m *Manager) AllTenants() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.tenants))
	for id := range m.tenants {
		ids = append(ids, id)
	}
	return ids
}

// GetTenantInfo returns full tenant info including mode and agent count.
func (m *Manager) GetTenantInfo(tenantID string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenant, exists := m.tenants[tenantID]
	if !exists {
		return nil
	}

	return map[string]interface{}{
		"id":          tenant.ID,
		"mode":        string(tenant.Mode),
		"agent_count": len(tenant.Agents),
	}
}

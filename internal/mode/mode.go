// Package mode manages the operating mode of the shield router.
package mode

import (
	"sync"
)

// Mode represents the operating mode of the shield.
type Mode string

const (
	// Enforce is normal operation - block matching requests.
	Enforce Mode = "enforce"
	// Audit logs all requests but never blocks (testing/debugging).
	Audit Mode = "audit"
	// Lockdown blocks ALL outbound requests (emergency).
	Lockdown Mode = "lockdown"
)

// Manager handles global and per-agent mode settings.
type Manager struct {
	mu          sync.RWMutex
	globalMode  Mode
	agentModes  map[string]Mode // agent ID -> mode override
}

// NewManager creates a new mode manager with enforce as default.
func NewManager() *Manager {
	return &Manager{
		globalMode: Enforce,
		agentModes: make(map[string]Mode),
	}
}

// GlobalMode returns the current global mode.
func (m *Manager) GlobalMode() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.globalMode
}

// SetGlobalMode sets the global operating mode.
func (m *Manager) SetGlobalMode(mode Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.globalMode = mode
}

// AgentMode returns the effective mode for an agent.
// Returns the agent-specific override if set, otherwise global mode.
func (m *Manager) AgentMode(agentID string) Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if mode, ok := m.agentModes[agentID]; ok {
		return mode
	}
	return m.globalMode
}

// SetAgentMode sets a mode override for a specific agent.
func (m *Manager) SetAgentMode(agentID string, mode Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agentModes[agentID] = mode
}

// ClearAgentMode removes the mode override for an agent (reverts to global).
func (m *Manager) ClearAgentMode(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.agentModes, agentID)
}

// AllAgentModes returns a copy of all agent mode overrides.
func (m *Manager) AllAgentModes() map[string]Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]Mode, len(m.agentModes))
	for k, v := range m.agentModes {
		result[k] = v
	}
	return result
}

// ShouldBlock returns whether a request should be blocked based on mode.
// In Audit mode, always returns false (log only).
// In Lockdown mode, always returns true (block everything).
// In Enforce mode, returns the provided ruleMatched value.
func (m *Manager) ShouldBlock(agentID string, ruleMatched bool) bool {
	mode := m.AgentMode(agentID)
	switch mode {
	case Audit:
		return false // Never block in audit mode
	case Lockdown:
		return true // Always block in lockdown mode
	default:
		return ruleMatched // Enforce: block only if rule matched
	}
}

// IsAudit returns whether the agent is in audit mode (for logging).
func (m *Manager) IsAudit(agentID string) bool {
	return m.AgentMode(agentID) == Audit
}

package mode

import (
	"sync"
	"testing"
)

func TestNewManager_DefaultModeIsEnforce(t *testing.T) {
	m := NewManager()

	if got := m.GlobalMode(); got != Enforce {
		t.Errorf("NewManager() GlobalMode = %q, want %q", got, Enforce)
	}
}

func TestSetGlobalMode(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
	}{
		{"set to audit", Audit},
		{"set to lockdown", Lockdown},
		{"set to enforce", Enforce},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			m.SetGlobalMode(tt.mode)

			if got := m.GlobalMode(); got != tt.mode {
				t.Errorf("GlobalMode() = %q, want %q", got, tt.mode)
			}
		})
	}
}

func TestAgentModeOverride(t *testing.T) {
	m := NewManager()
	agentID := "agent-123"

	// Without override, should return global mode
	if got := m.AgentMode(agentID); got != Enforce {
		t.Errorf("AgentMode() without override = %q, want %q", got, Enforce)
	}

	// Set agent override to Audit
	m.SetAgentMode(agentID, Audit)
	if got := m.AgentMode(agentID); got != Audit {
		t.Errorf("AgentMode() with override = %q, want %q", got, Audit)
	}

	// Another agent should still use global
	otherAgent := "agent-456"
	if got := m.AgentMode(otherAgent); got != Enforce {
		t.Errorf("AgentMode() for other agent = %q, want %q", got, Enforce)
	}

	// Change global mode - agent with override unaffected
	m.SetGlobalMode(Lockdown)
	if got := m.AgentMode(agentID); got != Audit {
		t.Errorf("AgentMode() after global change = %q, want %q", got, Audit)
	}
	if got := m.AgentMode(otherAgent); got != Lockdown {
		t.Errorf("AgentMode() for other agent after global change = %q, want %q", got, Lockdown)
	}
}

func TestClearAgentMode(t *testing.T) {
	m := NewManager()
	agentID := "agent-123"

	// Set override then clear it
	m.SetAgentMode(agentID, Audit)
	m.ClearAgentMode(agentID)

	// Should revert to global mode
	if got := m.AgentMode(agentID); got != Enforce {
		t.Errorf("AgentMode() after clear = %q, want %q", got, Enforce)
	}

	// Change global and verify agent follows
	m.SetGlobalMode(Lockdown)
	if got := m.AgentMode(agentID); got != Lockdown {
		t.Errorf("AgentMode() after clear and global change = %q, want %q", got, Lockdown)
	}
}

func TestShouldBlock(t *testing.T) {
	tests := []struct {
		name        string
		mode        Mode
		ruleMatched bool
		want        bool
	}{
		// Enforce mode: returns ruleMatched value
		{"enforce with rule matched", Enforce, true, true},
		{"enforce with rule not matched", Enforce, false, false},
		// Audit mode: always false
		{"audit with rule matched", Audit, true, false},
		{"audit with rule not matched", Audit, false, false},
		// Lockdown mode: always true
		{"lockdown with rule matched", Lockdown, true, true},
		{"lockdown with rule not matched", Lockdown, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			agentID := "test-agent"
			m.SetAgentMode(agentID, tt.mode)

			if got := m.ShouldBlock(agentID, tt.ruleMatched); got != tt.want {
				t.Errorf("ShouldBlock(%q, %v) = %v, want %v", agentID, tt.ruleMatched, got, tt.want)
			}
		})
	}
}

func TestShouldBlock_UsesAgentMode(t *testing.T) {
	m := NewManager()
	m.SetGlobalMode(Lockdown) // Global is lockdown

	agentID := "agent-123"
	m.SetAgentMode(agentID, Audit) // Agent override is audit

	// Agent should use audit (always false), not global lockdown
	if got := m.ShouldBlock(agentID, true); got != false {
		t.Errorf("ShouldBlock() with agent override = %v, want false", got)
	}

	// Another agent without override uses global lockdown
	if got := m.ShouldBlock("other-agent", false); got != true {
		t.Errorf("ShouldBlock() without override = %v, want true", got)
	}
}

func TestIsAudit(t *testing.T) {
	m := NewManager()
	agentID := "agent-123"

	// Default is enforce, not audit
	if m.IsAudit(agentID) {
		t.Error("IsAudit() with enforce mode = true, want false")
	}

	// Set to audit
	m.SetAgentMode(agentID, Audit)
	if !m.IsAudit(agentID) {
		t.Error("IsAudit() with audit mode = false, want true")
	}

	// Lockdown is not audit
	m.SetAgentMode(agentID, Lockdown)
	if m.IsAudit(agentID) {
		t.Error("IsAudit() with lockdown mode = true, want false")
	}
}

func TestAllAgentModes(t *testing.T) {
	m := NewManager()

	// Empty initially
	modes := m.AllAgentModes()
	if len(modes) != 0 {
		t.Errorf("AllAgentModes() initial len = %d, want 0", len(modes))
	}

	// Add some overrides
	m.SetAgentMode("agent-1", Audit)
	m.SetAgentMode("agent-2", Lockdown)

	modes = m.AllAgentModes()
	if len(modes) != 2 {
		t.Errorf("AllAgentModes() len = %d, want 2", len(modes))
	}
	if modes["agent-1"] != Audit {
		t.Errorf("AllAgentModes()[agent-1] = %q, want %q", modes["agent-1"], Audit)
	}
	if modes["agent-2"] != Lockdown {
		t.Errorf("AllAgentModes()[agent-2] = %q, want %q", modes["agent-2"], Lockdown)
	}

	// Verify it's a copy (modifying returned map doesn't affect manager)
	modes["agent-3"] = Enforce
	if len(m.AllAgentModes()) != 2 {
		t.Error("AllAgentModes() returned map was not a copy")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()
	const numGoroutines = 100
	const numIterations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			agentID := "agent"

			for j := 0; j < numIterations; j++ {
				// Mix of read and write operations
				switch j % 6 {
				case 0:
					m.SetGlobalMode(Audit)
				case 1:
					m.GlobalMode()
				case 2:
					m.SetAgentMode(agentID, Lockdown)
				case 3:
					m.AgentMode(agentID)
				case 4:
					m.ShouldBlock(agentID, true)
				case 5:
					m.ClearAgentMode(agentID)
				}
			}
		}(i)
	}

	wg.Wait()
	// Test passes if no race conditions occurred
}

func TestConcurrentAgentModes(t *testing.T) {
	m := NewManager()
	const numAgents = 50
	const numOps = 100

	var wg sync.WaitGroup
	wg.Add(numAgents * 2)

	// Writers
	for i := 0; i < numAgents; i++ {
		go func(agentNum int) {
			defer wg.Done()
			agentID := string(rune('A' + agentNum%26))

			for j := 0; j < numOps; j++ {
				modes := []Mode{Enforce, Audit, Lockdown}
				m.SetAgentMode(agentID, modes[j%3])
			}
		}(i)
	}

	// Readers
	for i := 0; i < numAgents; i++ {
		go func(agentNum int) {
			defer wg.Done()
			agentID := string(rune('A' + agentNum%26))

			for j := 0; j < numOps; j++ {
				m.AgentMode(agentID)
				m.ShouldBlock(agentID, j%2 == 0)
				m.AllAgentModes()
			}
		}(i)
	}

	wg.Wait()
}

package fleet

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if len(m.tenants) != 0 {
		t.Errorf("expected empty tenants, got %d", len(m.tenants))
	}
}

func TestCreateTenant(t *testing.T) {
	m := NewManager()
	tenant := m.CreateTenant("test-tenant")
	
	if tenant.ID != "test-tenant" {
		t.Errorf("expected ID 'test-tenant', got '%s'", tenant.ID)
	}
	if tenant.Mode != Isolated {
		t.Errorf("expected mode Isolated, got '%s'", tenant.Mode)
	}
}

func TestSetMode(t *testing.T) {
	m := NewManager()
	m.SetMode("tenant1", Fleet)
	
	if m.GetMode("tenant1") != Fleet {
		t.Errorf("expected Fleet mode")
	}
	
	m.SetMode("tenant1", Isolated)
	if m.GetMode("tenant1") != Isolated {
		t.Errorf("expected Isolated mode")
	}
}

func TestDefaultModeIsIsolated(t *testing.T) {
	m := NewManager()
	mode := m.GetMode("nonexistent")
	
	if mode != Isolated {
		t.Errorf("expected Isolated as default, got '%s'", mode)
	}
}

func TestAddAgent(t *testing.T) {
	m := NewManager()
	
	agent := Agent{
		ID:   "agent-1",
		Name: "Test Agent",
		IP:   "1.2.3.4",
	}
	
	m.AddAgent("tenant1", agent)
	
	// Should be able to look up agent's tenant
	tenantID := m.GetTenantForAgent("agent-1")
	if tenantID != "tenant1" {
		t.Errorf("expected tenant1, got '%s'", tenantID)
	}
}

func TestGetAgentsIsolatedMode(t *testing.T) {
	m := NewManager()
	m.AddAgent("tenant1", Agent{ID: "agent-1", Name: "Agent 1"})
	m.AddAgent("tenant1", Agent{ID: "agent-2", Name: "Agent 2"})
	
	// Default is isolated - should return empty
	agents := m.GetAgents("tenant1")
	if len(agents) != 0 {
		t.Errorf("expected empty agents in isolated mode, got %d", len(agents))
	}
}

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

func TestCanCommunicateIsolated(t *testing.T) {
	m := NewManager()
	m.AddAgent("tenant1", Agent{ID: "agent-1"})
	m.AddAgent("tenant1", Agent{ID: "agent-2"})
	// Default mode is isolated
	
	if m.CanCommunicate("agent-1", "agent-2") {
		t.Error("agents should not be able to communicate in isolated mode")
	}
}

func TestCanCommunicateFleet(t *testing.T) {
	m := NewManager()
	m.AddAgent("tenant1", Agent{ID: "agent-1"})
	m.AddAgent("tenant1", Agent{ID: "agent-2"})
	m.SetMode("tenant1", Fleet)
	
	if !m.CanCommunicate("agent-1", "agent-2") {
		t.Error("agents should be able to communicate in fleet mode")
	}
}

func TestCanCommunicateDifferentTenants(t *testing.T) {
	m := NewManager()
	m.AddAgent("tenant1", Agent{ID: "agent-1"})
	m.AddAgent("tenant2", Agent{ID: "agent-2"})
	m.SetMode("tenant1", Fleet)
	m.SetMode("tenant2", Fleet)
	
	// Even in fleet mode, agents from different tenants can't communicate
	if m.CanCommunicate("agent-1", "agent-2") {
		t.Error("agents from different tenants should not communicate")
	}
}

func TestGetAgentsForAgent(t *testing.T) {
	m := NewManager()
	m.AddAgent("tenant1", Agent{ID: "agent-1", Name: "Agent 1"})
	m.AddAgent("tenant1", Agent{ID: "agent-2", Name: "Agent 2"})
	m.AddAgent("tenant1", Agent{ID: "agent-3", Name: "Agent 3"})
	m.SetMode("tenant1", Fleet)
	
	// Should return other agents, not self
	agents := m.GetAgentsForAgent("agent-1")
	if len(agents) != 2 {
		t.Errorf("expected 2 other agents, got %d", len(agents))
	}
	
	for _, a := range agents {
		if a.ID == "agent-1" {
			t.Error("should not include self in agent list")
		}
	}
}

func TestRemoveAgent(t *testing.T) {
	m := NewManager()
	m.AddAgent("tenant1", Agent{ID: "agent-1"})
	m.SetMode("tenant1", Fleet)
	
	m.RemoveAgent("tenant1", "agent-1")
	
	if m.GetTenantForAgent("agent-1") != "" {
		t.Error("agent should be removed")
	}
	
	agents := m.GetAgents("tenant1")
	if len(agents) != 0 {
		t.Errorf("expected 0 agents after removal, got %d", len(agents))
	}
}

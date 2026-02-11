package fleet

import (
	"testing"
)

func TestValidateAgentIP(t *testing.T) {
	mgr := NewManager()
	mgr.CreateTenant("test-tenant")
	mgr.AddAgent("test-tenant", Agent{
		ID:   "agent-1",
		IP:   "192.168.1.100",
		Tier: "crew",
	})
	mgr.AddAgent("test-tenant", Agent{
		ID:   "agent-2",
		IP:   "10.0.0.50",
		Tier: "commodore",
	})

	tests := []struct {
		name         string
		ip           string
		wantID       string
		wantTier     string
		wantValid    bool
	}{
		{
			name:      "registered crew agent",
			ip:        "192.168.1.100",
			wantID:    "agent-1",
			wantTier:  "crew",
			wantValid: true,
		},
		{
			name:      "registered commodore agent",
			ip:        "10.0.0.50",
			wantID:    "agent-2",
			wantTier:  "commodore",
			wantValid: true,
		},
		{
			name:      "unregistered IP",
			ip:        "1.2.3.4",
			wantID:    "",
			wantTier:  "",
			wantValid: false,
		},
		{
			name:      "empty IP",
			ip:        "",
			wantID:    "",
			wantTier:  "",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, tier, valid := mgr.ValidateAgentIP(tt.ip)
			if id != tt.wantID {
				t.Errorf("id = %q, want %q", id, tt.wantID)
			}
			if tier != tt.wantTier {
				t.Errorf("tier = %q, want %q", tier, tt.wantTier)
			}
			if valid != tt.wantValid {
				t.Errorf("valid = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

func TestValidateAgentIP_DefaultTier(t *testing.T) {
	mgr := NewManager()
	mgr.CreateTenant("test")
	mgr.AddAgent("test", Agent{
		ID: "no-tier-agent",
		IP: "192.168.1.1",
		// Tier not set
	})

	_, tier, valid := mgr.ValidateAgentIP("192.168.1.1")
	if !valid {
		t.Error("expected valid")
	}
	if tier != "crew" {
		t.Errorf("expected default tier 'crew', got %q", tier)
	}
}

func TestValidateAgentIP_RemoveAgent(t *testing.T) {
	mgr := NewManager()
	mgr.CreateTenant("test")
	mgr.AddAgent("test", Agent{
		ID:   "temp-agent",
		IP:   "192.168.1.99",
		Tier: "crew",
	})

	// Verify agent exists
	_, _, valid := mgr.ValidateAgentIP("192.168.1.99")
	if !valid {
		t.Error("agent should exist")
	}

	// Remove agent
	mgr.RemoveAgent("test", "temp-agent")

	// Verify agent no longer validates
	_, _, valid = mgr.ValidateAgentIP("192.168.1.99")
	if valid {
		t.Error("agent should no longer exist after removal")
	}
}

func TestGetAgentByIP(t *testing.T) {
	mgr := NewManager()
	mgr.CreateTenant("test")
	mgr.AddAgent("test", Agent{
		ID:   "my-agent",
		Name: "My Agent",
		IP:   "10.20.30.40",
		Tier: "captain",
	})

	agent := mgr.GetAgentByIP("10.20.30.40")
	if agent == nil {
		t.Fatal("expected agent, got nil")
	}
	if agent.ID != "my-agent" {
		t.Errorf("expected ID 'my-agent', got %q", agent.ID)
	}
	if agent.Name != "My Agent" {
		t.Errorf("expected Name 'My Agent', got %q", agent.Name)
	}

	// Non-existent IP
	agent = mgr.GetAgentByIP("1.1.1.1")
	if agent != nil {
		t.Errorf("expected nil for unknown IP, got %+v", agent)
	}
}

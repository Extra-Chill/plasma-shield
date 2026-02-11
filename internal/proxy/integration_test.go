package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Extra-Chill/plasma-shield/internal/fleet"
	"github.com/Extra-Chill/plasma-shield/internal/mode"
	"github.com/Extra-Chill/plasma-shield/internal/rules"
)

// TestIntegration_ForwardProxyWithRegistry tests the full forward proxy flow
func TestIntegration_ForwardProxyWithRegistry(t *testing.T) {
	// Create a destination server
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("destination reached"))
	}))
	defer destination.Close()

	// Set up fleet with registered agent
	fleetMgr := fleet.NewManager()
	fleetMgr.CreateTenant("test-tenant")
	fleetMgr.AddAgent("test-tenant", fleet.Agent{
		ID:   "agent-1",
		IP:   "192.168.1.100",
		Tier: "crew",
	})

	// Set up rules engine
	engine := rules.NewEngine()
	modeManager := mode.NewManager()
	inspector := NewInspector(engine, modeManager)

	// Create handler with registry
	handler := NewHandler(inspector, WithAgentRegistry(fleetMgr))

	t.Run("registered agent can proxy", func(t *testing.T) {
		req := httptest.NewRequest("GET", destination.URL, nil)
		req.RemoteAddr = "192.168.1.100:12345" // Registered IP
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("unregistered agent is blocked", func(t *testing.T) {
		req := httptest.NewRequest("GET", destination.URL, nil)
		req.RemoteAddr = "10.0.0.99:12345" // Unregistered IP
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})
}

// TestIntegration_TierBasedFiltering tests tier-aware rule enforcement
func TestIntegration_TierBasedFiltering(t *testing.T) {
	// Set up fleet with agents of different tiers
	fleetMgr := fleet.NewManager()
	fleetMgr.CreateTenant("test-tenant")
	fleetMgr.AddAgent("test-tenant", fleet.Agent{
		ID:   "crew-agent",
		IP:   "192.168.1.10",
		Tier: "crew",
	})
	fleetMgr.AddAgent("test-tenant", fleet.Agent{
		ID:   "commodore-agent",
		IP:   "192.168.1.20",
		Tier: "commodore",
	})

	// Set up rules - block Hetzner for non-commodore
	engine := rules.NewEngine()
	engine.LoadRulesFromBytes([]byte(`
rules:
  - id: block-hetzner
    domain: "api.hetzner.cloud"
    action: block
    tiers: [crew, captain]
    enabled: true
`))

	modeManager := mode.NewManager()
	inspector := NewInspector(engine, modeManager)
	handler := NewHandler(inspector, WithAgentRegistry(fleetMgr))

	t.Run("crew blocked from Hetzner", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://api.hetzner.cloud/v1/servers", nil)
		req.RemoteAddr = "192.168.1.10:12345" // Crew agent
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("crew should be blocked from Hetzner, got %d", rr.Code)
		}
	})

	t.Run("commodore allowed to Hetzner", func(t *testing.T) {
		// Note: This will fail to connect (no real server) but should NOT be blocked by rules
		req := httptest.NewRequest("GET", "http://api.hetzner.cloud/v1/servers", nil)
		req.RemoteAddr = "192.168.1.20:12345" // Commodore agent
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Should get BadGateway (can't connect) not Forbidden (blocked)
		if rr.Code == http.StatusForbidden {
			t.Errorf("commodore should NOT be blocked from Hetzner")
		}
	})
}

// TestIntegration_ReverseProxyTenantIsolation tests tenant isolation
func TestIntegration_ReverseProxyTenantIsolation(t *testing.T) {
	// Create backend agents
	agent1Backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("agent1"))
	}))
	defer agent1Backend.Close()

	agent2Backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("agent2"))
	}))
	defer agent2Backend.Close()

	// Set up two tenants with their own agents
	fleetMgr := fleet.NewManager()

	fleetMgr.CreateTenant("tenant-a")
	fleetMgr.AddAgent("tenant-a", fleet.Agent{
		ID:         "agent1",
		WebhookURL: agent1Backend.URL,
	})

	fleetMgr.CreateTenant("tenant-b")
	fleetMgr.AddAgent("tenant-b", fleet.Agent{
		ID:         "agent2",
		WebhookURL: agent2Backend.URL,
	})

	handler := NewReverseHandler(fleetMgr)
	handler.RegisterToken("token-a", "tenant-a")
	handler.RegisterToken("token-b", "tenant-b")

	t.Run("tenant A can access own agent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/agent/agent1/test", nil)
		req.Header.Set("Authorization", "Bearer token-a")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if rr.Body.String() != "agent1" {
			t.Errorf("expected 'agent1', got '%s'", rr.Body.String())
		}
	})

	t.Run("tenant A cannot access tenant B agent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/agent/agent2/test", nil)
		req.Header.Set("Authorization", "Bearer token-a")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("tenant B can access own agent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/agent/agent2/test", nil)
		req.Header.Set("Authorization", "Bearer token-b")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if rr.Body.String() != "agent2" {
			t.Errorf("expected 'agent2', got '%s'", rr.Body.String())
		}
	})

	t.Run("tenant B cannot access tenant A agent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/agent/agent1/test", nil)
		req.Header.Set("Authorization", "Bearer token-b")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})
}

// TestIntegration_FullFlowSimulation simulates Captain -> Shield -> Agent
func TestIntegration_FullFlowSimulation(t *testing.T) {
	// This simulates the full flow:
	// Captain (Chubes) -> Shield (reverse proxy) -> Agent (Sarai)
	// Agent should see request from "Captain" not from original source

	var captainSeen string
	var forwardedForSeen string

	agentBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captainSeen = r.Header.Get("X-Captain")
		forwardedForSeen = r.Header.Get("X-Forwarded-For")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"task received from captain"}`))
	}))
	defer agentBackend.Close()

	fleetMgr := fleet.NewManager()
	fleetMgr.CreateTenant("chubes-fleet")
	fleetMgr.SetCaptainName("chubes-fleet", "Chubes")
	fleetMgr.AddAgent("chubes-fleet", fleet.Agent{
		ID:         "sarai",
		Name:       "Sarai Chinwag",
		WebhookURL: agentBackend.URL,
		Tier:       "crew",
	})

	handler := NewReverseHandler(fleetMgr)
	handler.RegisterToken("chubes-token", "chubes-fleet")

	// Simulate request from Fleet Command (another agent) to Sarai
	req := httptest.NewRequest("POST", "/agent/sarai/hooks", nil)
	req.Header.Set("Authorization", "Bearer chubes-token")
	req.Header.Set("X-Forwarded-For", "178.156.153.244") // Fleet Command IP
	req.Header.Set("X-Agent-Id", "fleet-command")        // Source agent
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify identity masking
	if captainSeen != "Chubes" {
		t.Errorf("agent should see Captain='Chubes', got '%s'", captainSeen)
	}

	if forwardedForSeen != "" {
		t.Errorf("X-Forwarded-For should be stripped (identity masking), got '%s'", forwardedForSeen)
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

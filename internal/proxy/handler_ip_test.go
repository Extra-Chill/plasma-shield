package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Extra-Chill/plasma-shield/internal/mode"
	"github.com/Extra-Chill/plasma-shield/internal/rules"
)

// mockRegistry implements AgentRegistry for testing.
type mockRegistry struct {
	agents map[string]struct {
		id   string
		tier string
	}
}

func (m *mockRegistry) ValidateAgentIP(ip string) (agentID string, tier string, valid bool) {
	if agent, ok := m.agents[ip]; ok {
		return agent.id, agent.tier, true
	}
	return "", "", false
}

func TestHandler_RejectsUnregisteredIP(t *testing.T) {
	// Create handler with a registry that has no agents
	registry := &mockRegistry{
		agents: make(map[string]struct {
			id   string
			tier string
		}),
	}

	engine := rules.NewEngine()
	modeManager := mode.NewManager()
	inspector := NewInspector(engine, modeManager)
	handler := NewHandler(inspector, WithAgentRegistry(registry))

	// Make request from unregistered IP
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden, got %d", rr.Code)
	}
}

func TestHandler_AllowsRegisteredIP(t *testing.T) {
	// Create a backend to proxy to
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backend.Close()

	// Create handler with a registry that has the test IP
	registry := &mockRegistry{
		agents: map[string]struct {
			id   string
			tier string
		}{
			"192.168.1.100": {id: "test-agent", tier: "crew"},
		},
	}

	engine := rules.NewEngine()
	modeManager := mode.NewManager()
	inspector := NewInspector(engine, modeManager)
	handler := NewHandler(inspector, WithAgentRegistry(registry))

	// Make request from registered IP
	req := httptest.NewRequest("GET", backend.URL+"/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rr.Code)
	}
}

func TestHandler_NoRegistryAllowsAll(t *testing.T) {
	// Create a backend to proxy to
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backend.Close()

	// Create handler WITHOUT a registry (backwards compatibility)
	engine := rules.NewEngine()
	modeManager := mode.NewManager()
	inspector := NewInspector(engine, modeManager)
	handler := NewHandler(inspector) // No registry

	// Make request from any IP
	req := httptest.NewRequest("GET", backend.URL+"/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK (no registry = allow all), got %d", rr.Code)
	}
}

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		remoteAddr string
		expected   string
	}{
		{"192.168.1.1:12345", "192.168.1.1"},
		{"10.0.0.1:443", "10.0.0.1"},
		{"[::1]:8080", "::1"},
		{"127.0.0.1:0", "127.0.0.1"},
		{"invalid", "invalid"}, // Falls through if no port
	}

	for _, tt := range tests {
		t.Run(tt.remoteAddr, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			req.RemoteAddr = tt.remoteAddr
			got := extractClientIP(req)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

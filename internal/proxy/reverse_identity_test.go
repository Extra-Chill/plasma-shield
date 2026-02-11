package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Extra-Chill/plasma-shield/internal/fleet"
)

// TestIdentityMasking_HeadersStripped verifies that identity-revealing headers are removed
func TestIdentityMasking_HeadersStripped(t *testing.T) {
	headersToStrip := []string{
		"X-Forwarded-For",
		"X-Real-Ip",
		"X-Originating-Ip",
		"X-Remote-Ip",
		"X-Remote-Addr",
		"X-Client-Ip",
		"X-Agent-Id",
		"X-Source-Agent",
	}

	for _, header := range headersToStrip {
		t.Run(header, func(t *testing.T) {
			var receivedHeader string
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedHeader = r.Header.Get(header)
				w.WriteHeader(http.StatusOK)
			}))
			defer backend.Close()

			fleetMgr := fleet.NewManager()
			fleetMgr.CreateTenant("tenant1")
			fleetMgr.SetCaptainName("tenant1", "Captain Test")
			fleetMgr.AddAgent("tenant1", fleet.Agent{
				ID:         "agent1",
				WebhookURL: backend.URL,
			})

			handler := NewReverseHandler(fleetMgr)
			handler.RegisterToken("token1", "tenant1")

			req := httptest.NewRequest("POST", "/agent/agent1/test", nil)
			req.Header.Set("Authorization", "Bearer token1")
			req.Header.Set(header, "should-be-stripped")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if receivedHeader != "" {
				t.Errorf("header %s should be stripped, but got value: %s", header, receivedHeader)
			}
		})
	}
}

// TestIdentityMasking_CaptainHeaderSet verifies X-Captain header is set correctly
func TestIdentityMasking_CaptainHeaderSet(t *testing.T) {
	tests := []struct {
		name         string
		captainName  string
		tenantID     string
		expectedName string
	}{
		{
			name:         "uses captain name when set",
			captainName:  "Admiral Ackbar",
			tenantID:     "rebels",
			expectedName: "Admiral Ackbar",
		},
		{
			name:         "falls back to tenant ID when no captain name",
			captainName:  "",
			tenantID:     "empire",
			expectedName: "empire",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedCaptain string
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedCaptain = r.Header.Get("X-Captain")
				w.WriteHeader(http.StatusOK)
			}))
			defer backend.Close()

			fleetMgr := fleet.NewManager()
			fleetMgr.CreateTenant(tt.tenantID)
			if tt.captainName != "" {
				fleetMgr.SetCaptainName(tt.tenantID, tt.captainName)
			}
			fleetMgr.AddAgent(tt.tenantID, fleet.Agent{
				ID:         "agent1",
				WebhookURL: backend.URL,
			})

			handler := NewReverseHandler(fleetMgr)
			handler.RegisterToken("token1", tt.tenantID)

			req := httptest.NewRequest("GET", "/agent/agent1/status", nil)
			req.Header.Set("Authorization", "Bearer token1")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if receivedCaptain != tt.expectedName {
				t.Errorf("expected X-Captain '%s', got '%s'", tt.expectedName, receivedCaptain)
			}
		})
	}
}

// TestIdentityMasking_PreservesLegitimateHeaders verifies non-identity headers pass through
func TestIdentityMasking_PreservesLegitimateHeaders(t *testing.T) {
	headersToPreserve := map[string]string{
		"Content-Type":    "application/json",
		"Accept":          "text/html",
		"User-Agent":      "TestClient/1.0",
		"X-Custom-Header": "custom-value",
		"Accept-Language": "en-US",
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for header, expected := range headersToPreserve {
			got := r.Header.Get(header)
			if got != expected {
				http.Error(w, "header mismatch: "+header, http.StatusBadRequest)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	fleetMgr := fleet.NewManager()
	fleetMgr.CreateTenant("tenant1")
	fleetMgr.AddAgent("tenant1", fleet.Agent{
		ID:         "agent1",
		WebhookURL: backend.URL,
	})

	handler := NewReverseHandler(fleetMgr)
	handler.RegisterToken("token1", "tenant1")

	req := httptest.NewRequest("POST", "/agent/agent1/test", nil)
	req.Header.Set("Authorization", "Bearer token1")
	for header, value := range headersToPreserve {
		req.Header.Set(header, value)
	}
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// TestIdentityMasking_RequestBodyPreserved verifies request body passes through
func TestIdentityMasking_RequestBodyPreserved(t *testing.T) {
	expectedBody := `{"command": "do something", "data": [1, 2, 3]}`

	var receivedBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	fleetMgr := fleet.NewManager()
	fleetMgr.CreateTenant("tenant1")
	fleetMgr.AddAgent("tenant1", fleet.Agent{
		ID:         "agent1",
		WebhookURL: backend.URL,
	})

	handler := NewReverseHandler(fleetMgr)
	handler.RegisterToken("token1", "tenant1")

	req := httptest.NewRequest("POST", "/agent/agent1/hooks", strings.NewReader(expectedBody))
	req.Header.Set("Authorization", "Bearer token1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if receivedBody != expectedBody {
		t.Errorf("body not preserved.\nexpected: %s\ngot: %s", expectedBody, receivedBody)
	}
}

// TestIdentityMasking_ResponsePreserved verifies response passes back correctly
func TestIdentityMasking_ResponsePreserved(t *testing.T) {
	expectedBody := `{"status": "success", "agent": "masked"}`
	expectedHeaders := map[string]string{
		"Content-Type":     "application/json",
		"X-Custom-Response": "test-value",
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range expectedHeaders {
			w.Header().Set(k, v)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(expectedBody))
	}))
	defer backend.Close()

	fleetMgr := fleet.NewManager()
	fleetMgr.CreateTenant("tenant1")
	fleetMgr.AddAgent("tenant1", fleet.Agent{
		ID:         "agent1",
		WebhookURL: backend.URL,
	})

	handler := NewReverseHandler(fleetMgr)
	handler.RegisterToken("token1", "tenant1")

	req := httptest.NewRequest("POST", "/agent/agent1/action", nil)
	req.Header.Set("Authorization", "Bearer token1")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	for k, expected := range expectedHeaders {
		got := rr.Header().Get(k)
		if got != expected {
			t.Errorf("response header %s: expected '%s', got '%s'", k, expected, got)
		}
	}

	if rr.Body.String() != expectedBody {
		t.Errorf("response body: expected '%s', got '%s'", expectedBody, rr.Body.String())
	}
}

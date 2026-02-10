package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Extra-Chill/plasma-shield/internal/mode"
	"github.com/Extra-Chill/plasma-shield/internal/rules"
)

// testInspector creates an inspector with the given rules YAML.
func testInspector(t *testing.T, rulesYAML string) *Inspector {
	t.Helper()
	engine := rules.NewEngine()
	if rulesYAML != "" {
		if err := engine.LoadRulesFromBytes([]byte(rulesYAML)); err != nil {
			t.Fatalf("failed to load rules: %v", err)
		}
	}
	modeManager := mode.NewManager()
	return NewInspector(engine, modeManager)
}

// captureLog captures log output during test execution.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(io.Discard)
	fn()
	return buf.String()
}

func TestHandleHTTP_AllowedRequests(t *testing.T) {
	// Create a mock upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Upstream", "reached")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("upstream response"))
	}))
	defer upstream.Close()

	inspector := testInspector(t, `
rules:
  - id: block-evil
    domain: "evil.com"
    action: block
    description: "Block evil domain"
    enabled: true
`)

	handler := NewHandler(inspector)

	tests := []struct {
		name           string
		method         string
		url            string
		wantStatus     int
		wantBodySubstr string
	}{
		{
			name:           "GET allowed domain",
			method:         http.MethodGet,
			url:            upstream.URL + "/test",
			wantStatus:     http.StatusOK,
			wantBodySubstr: "upstream response",
		},
		{
			name:           "POST allowed domain",
			method:         http.MethodPost,
			url:            upstream.URL + "/submit",
			wantStatus:     http.StatusOK,
			wantBodySubstr: "upstream response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBodySubstr) {
				t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantBodySubstr)
			}
		})
	}
}

func TestHandleHTTP_BlockedRequests(t *testing.T) {
	inspector := testInspector(t, `
rules:
  - id: block-pastebin
    domain: "pastebin.com"
    action: block
    description: "Block pastebin"
    enabled: true
  - id: block-evil
    domain: "*.evil.net"
    action: block
    description: "Block evil subdomains"
    enabled: true
`)

	handler := NewHandler(inspector)

	tests := []struct {
		name       string
		host       string
		wantStatus int
		wantReason string
	}{
		{
			name:       "exact domain block",
			host:       "pastebin.com",
			wantStatus: http.StatusForbidden,
			wantReason: "Blocked by Plasma Shield",
		},
		{
			name:       "wildcard subdomain block",
			host:       "api.evil.net",
			wantStatus: http.StatusForbidden,
			wantReason: "Blocked by Plasma Shield",
		},
		{
			name:       "deep subdomain block",
			host:       "deep.sub.evil.net",
			wantStatus: http.StatusForbidden,
			wantReason: "Blocked by Plasma Shield",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://"+tt.host+"/path", nil)
			req.Host = tt.host
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantReason) {
				t.Errorf("body = %q, want to contain %q", rec.Body.String(), tt.wantReason)
			}
		})
	}
}

func TestHandleConnect_Blocked(t *testing.T) {
	inspector := testInspector(t, `
rules:
  - id: block-malware
    domain: "malware.site"
    action: block
    description: "Block malware site"
    enabled: true
`)

	handler := NewHandler(inspector)

	tests := []struct {
		name       string
		host       string
		wantStatus int
	}{
		{
			name:       "CONNECT to blocked domain",
			host:       "malware.site:443",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "CONNECT to blocked domain with port",
			host:       "malware.site:8443",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodConnect, tt.host, nil)
			req.Host = tt.host
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestHeaderCopying_NoAgentTokenLeak(t *testing.T) {
	var receivedHeaders http.Header

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	inspector := testInspector(t, "")
	handler := NewHandler(inspector)

	tests := []struct {
		name              string
		requestHeaders    map[string]string
		wantAbsent        []string
		wantPresent       []string
	}{
		{
			name: "X-Agent-Token stripped",
			requestHeaders: map[string]string{
				"X-Agent-Token":    "secret-token-123",
				"Authorization":    "Bearer xyz",
				"X-Custom-Header":  "keep-this",
			},
			wantAbsent:  []string{"X-Agent-Token"},
			wantPresent: []string{"Authorization", "X-Custom-Header"},
		},
		{
			name: "Proxy-Connection stripped",
			requestHeaders: map[string]string{
				"Proxy-Connection": "keep-alive",
				"Content-Type":     "application/json",
			},
			wantAbsent:  []string{"Proxy-Connection"},
			wantPresent: []string{"Content-Type"},
		},
		{
			name: "both proxy headers stripped",
			requestHeaders: map[string]string{
				"X-Agent-Token":    "token",
				"Proxy-Connection": "keep-alive",
				"Accept":           "text/html",
			},
			wantAbsent:  []string{"X-Agent-Token", "Proxy-Connection"},
			wantPresent: []string{"Accept"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedHeaders = nil

			req := httptest.NewRequest(http.MethodGet, upstream.URL+"/test", nil)
			for k, v := range tt.requestHeaders {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if receivedHeaders == nil {
				t.Fatal("upstream did not receive request")
			}

			for _, header := range tt.wantAbsent {
				if val := receivedHeaders.Get(header); val != "" {
					t.Errorf("header %q leaked to upstream with value %q", header, val)
				}
			}

			for _, header := range tt.wantPresent {
				if val := receivedHeaders.Get(header); val == "" {
					t.Errorf("header %q was not forwarded to upstream", header)
				}
			}
		})
	}
}

func TestRequestLogging(t *testing.T) {
	inspector := testInspector(t, `
rules:
  - id: block-test
    domain: "blocked.test"
    action: block
    description: "Test block"
    enabled: true
`)

	handler := NewHandler(inspector)

	tests := []struct {
		name        string
		host        string
		method      string
		agentToken  string
		wantAction  string
		wantDomain  string
	}{
		{
			name:       "allowed request logged",
			host:       "allowed.test",
			method:     http.MethodGet,
			agentToken: "agent-123",
			wantAction: "allow",
			wantDomain: "allowed.test",
		},
		{
			name:       "blocked request logged",
			host:       "blocked.test",
			method:     http.MethodGet,
			agentToken: "agent-456",
			wantAction: "block",
			wantDomain: "blocked.test",
		},
		{
			name:       "CONNECT logged",
			host:       "blocked.test:443",
			method:     http.MethodConnect,
			agentToken: "agent-789",
			wantAction: "block",
			wantDomain: "blocked.test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logOutput string

			req := httptest.NewRequest(tt.method, "http://"+tt.host+"/path", nil)
			req.Host = tt.host
			if tt.agentToken != "" {
				req.Header.Set("X-Agent-Token", tt.agentToken)
			}
			rec := httptest.NewRecorder()

			logOutput = captureLog(t, func() {
				handler.ServeHTTP(rec, req)
			})

			// Parse the JSON log entry
			var entry LogEntry
			// Extract JSON from log line (after timestamp prefix)
			jsonStart := strings.Index(logOutput, "{")
			if jsonStart == -1 {
				t.Fatalf("no JSON in log output: %q", logOutput)
			}
			jsonEnd := strings.LastIndex(logOutput, "}")
			if jsonEnd == -1 {
				t.Fatalf("malformed JSON in log output: %q", logOutput)
			}
			jsonStr := logOutput[jsonStart : jsonEnd+1]

			if err := json.Unmarshal([]byte(jsonStr), &entry); err != nil {
				t.Fatalf("failed to parse log entry: %v, json: %q", err, jsonStr)
			}

			if entry.Action != tt.wantAction {
				t.Errorf("action = %q, want %q", entry.Action, tt.wantAction)
			}
			if entry.Domain != tt.wantDomain {
				t.Errorf("domain = %q, want %q", entry.Domain, tt.wantDomain)
			}
			if entry.AgentToken != tt.agentToken {
				t.Errorf("agentToken = %q, want %q", entry.AgentToken, tt.agentToken)
			}
			if entry.Timestamp.IsZero() {
				t.Error("timestamp should not be zero")
			}
		})
	}
}

func TestLogEntryFormat(t *testing.T) {
	inspector := testInspector(t, `
rules:
  - id: block-reason-test
    domain: "reason.test"
    action: block
    description: "Testing reason field"
    enabled: true
`)

	handler := NewHandler(inspector)

	req := httptest.NewRequest(http.MethodGet, "http://reason.test/path", nil)
	req.Host = "reason.test"
	req.Header.Set("X-Agent-Token", "test-agent")
	rec := httptest.NewRecorder()

	logOutput := captureLog(t, func() {
		handler.ServeHTTP(rec, req)
	})

	// Verify log contains expected JSON fields
	if !strings.Contains(logOutput, `"action":"block"`) {
		t.Error("log should contain action field")
	}
	if !strings.Contains(logOutput, `"domain":"reason.test"`) {
		t.Error("log should contain domain field")
	}
	if !strings.Contains(logOutput, `"method":"GET"`) {
		t.Error("log should contain method field")
	}
	if !strings.Contains(logOutput, `"agent_token":"test-agent"`) {
		t.Error("log should contain agent_token field")
	}
	if !strings.Contains(logOutput, `"reason"`) {
		t.Error("log should contain reason field")
	}
}

func TestResponseHeaderCopying(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Response", "test-value")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer upstream.Close()

	inspector := testInspector(t, "")
	handler := NewHandler(inspector)

	req := httptest.NewRequest(http.MethodGet, upstream.URL+"/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	if custom := rec.Header().Get("X-Custom-Response"); custom != "test-value" {
		t.Errorf("X-Custom-Response = %q, want %q", custom, "test-value")
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", cc, "no-cache")
	}
	if body := rec.Body.String(); body != `{"status":"ok"}` {
		t.Errorf("body = %q, want %q", body, `{"status":"ok"}`)
	}
}

func TestCopyHeaders(t *testing.T) {
	tests := []struct {
		name string
		src  http.Header
		want http.Header
	}{
		{
			name: "single values",
			src: http.Header{
				"Content-Type":  []string{"application/json"},
				"Authorization": []string{"Bearer token"},
			},
			want: http.Header{
				"Content-Type":  []string{"application/json"},
				"Authorization": []string{"Bearer token"},
			},
		},
		{
			name: "multiple values",
			src: http.Header{
				"Accept-Encoding": []string{"gzip", "deflate"},
				"Cookie":          []string{"a=1", "b=2"},
			},
			want: http.Header{
				"Accept-Encoding": []string{"gzip", "deflate"},
				"Cookie":          []string{"a=1", "b=2"},
			},
		},
		{
			name: "empty headers",
			src:  http.Header{},
			want: http.Header{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := make(http.Header)
			copyHeaders(dst, tt.src)

			for key, wantVals := range tt.want {
				gotVals := dst[key]
				if len(gotVals) != len(wantVals) {
					t.Errorf("header %q: got %d values, want %d", key, len(gotVals), len(wantVals))
					continue
				}
				for i, wantVal := range wantVals {
					if gotVals[i] != wantVal {
						t.Errorf("header %q[%d]: got %q, want %q", key, i, gotVals[i], wantVal)
					}
				}
			}
		})
	}
}

func TestExecCheckHandler(t *testing.T) {
	engine := rules.NewEngine()
	if err := engine.LoadRulesFromBytes([]byte(`
rules:
  - id: block-rm-rf
    pattern: "rm -rf *"
    action: block
    description: "Block recursive delete"
    enabled: true
`)); err != nil {
		t.Fatalf("failed to load rules: %v", err)
	}

	modeManager := mode.NewManager()
	inspector := NewInspector(engine, modeManager)
	handler := NewExecCheckHandler(inspector)

	tests := []struct {
		name        string
		method      string
		body        string
		wantStatus  int
		wantAllowed bool
		wantReason  string
	}{
		{
			name:        "allowed command",
			method:      http.MethodPost,
			body:        `{"command": "ls -la", "agent_token": "test"}`,
			wantStatus:  http.StatusOK,
			wantAllowed: true,
		},
		{
			name:        "blocked command",
			method:      http.MethodPost,
			body:        `{"command": "rm -rf /", "agent_token": "test"}`,
			wantStatus:  http.StatusOK,
			wantAllowed: false,
			wantReason:  "block-rm-rf",
		},
		{
			name:       "wrong method",
			method:     http.MethodGet,
			body:       `{"command": "ls"}`,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "invalid JSON",
			method:     http.MethodPost,
			body:       `{invalid`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/exec/check", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// Discard logs during test
			log.SetOutput(io.Discard)
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp ExecCheckResponse
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Allowed != tt.wantAllowed {
					t.Errorf("allowed = %v, want %v", resp.Allowed, tt.wantAllowed)
				}
				if tt.wantReason != "" && !strings.Contains(resp.Reason, tt.wantReason) {
					t.Errorf("reason = %q, want to contain %q", resp.Reason, tt.wantReason)
				}
			}
		})
	}
}

func TestHostExtraction(t *testing.T) {
	inspector := testInspector(t, `
rules:
  - id: block-specific
    domain: "specific.domain.com"
    action: block
    description: "Block specific domain"
    enabled: true
`)

	handler := NewHandler(inspector)

	tests := []struct {
		name       string
		host       string
		urlHost    string
		wantStatus int
	}{
		{
			name:       "host from Host header",
			host:       "specific.domain.com",
			urlHost:    "",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "host with port stripped",
			host:       "specific.domain.com:8080",
			urlHost:    "",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "case insensitive",
			host:       "SPECIFIC.DOMAIN.COM",
			urlHost:    "",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "http://"
			if tt.urlHost != "" {
				url += tt.urlHost
			} else {
				url += tt.host
			}
			url += "/path"

			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Host = tt.host
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (host=%q)", rec.Code, tt.wantStatus, tt.host)
			}
		})
	}
}

func TestDomainBlockingIntegration(t *testing.T) {
	// Test various blocking patterns work correctly through the handler
	inspector := testInspector(t, `
rules:
  - id: block-exact
    domain: "exact.com"
    action: block
    description: "Exact match"
    enabled: true
  - id: block-wildcard
    domain: "*.wild.com"
    action: block
    description: "Wildcard subdomain"
    enabled: true
  - id: block-contains
    domain: "*xmr*"
    action: block
    description: "Contains pattern"
    enabled: true
`)

	handler := NewHandler(inspector)

	tests := []struct {
		name    string
		host    string
		blocked bool
	}{
		// Exact match
		{"exact match blocks", "exact.com", true},
		{"exact match subdomain allowed", "www.exact.com", false},

		// Wildcard
		{"wildcard subdomain blocks", "api.wild.com", true},
		{"wildcard deep subdomain blocks", "a.b.wild.com", true},
		{"wildcard base also blocks", "wild.com", true}, // *.wild.com pattern matches base too

		// Contains
		{"contains xmr blocks", "xmrpool.net", true},
		{"contains xmr blocks mid", "my.xmr.pool", true},

		// Allowed
		{"unmatched domain allowed", "google.com", false},
		{"unmatched domain allowed 2", "safe.example.org", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://"+tt.host+"/", nil)
			req.Host = tt.host
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			gotBlocked := rec.Code == http.StatusForbidden
			if gotBlocked != tt.blocked {
				t.Errorf("host %q: blocked=%v, want %v (status=%d)", tt.host, gotBlocked, tt.blocked, rec.Code)
			}
		})
	}
}

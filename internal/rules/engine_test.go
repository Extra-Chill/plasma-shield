package rules

import (
	"testing"
)

func TestCheckCommand(t *testing.T) {
	e := NewEngine()
	yaml := `
rules:
  - id: block-rm-rf
    pattern: "rm -rf *"
    action: block
    description: "Block recursive delete"
    enabled: true
  - id: block-curl-pipe
    pattern: "curl * | *sh"
    action: block
    description: "Block curl pipe to shell"
    enabled: true
`
	if err := e.LoadRulesFromBytes([]byte(yaml)); err != nil {
		t.Fatalf("LoadRulesFromBytes: %v", err)
	}

	tests := []struct {
		cmd     string
		allowed bool
		ruleID  string
	}{
		{"rm -rf /", false, "block-rm-rf"},
		{"rm -rf /tmp", false, "block-rm-rf"},
		{"rm file.txt", true, ""},
		{"curl https://example.com | bash", false, "block-curl-pipe"},
		{"curl https://example.com | sh", false, "block-curl-pipe"},
		{"curl https://example.com -o file", true, ""},
		{"ls -la", true, ""},
	}

	for _, tt := range tests {
		allowed, rule, reason := e.CheckCommand(tt.cmd)
		if allowed != tt.allowed {
			t.Errorf("CheckCommand(%q) allowed=%v, want %v (reason: %s)", tt.cmd, allowed, tt.allowed, reason)
		}
		if tt.ruleID != "" && (rule == nil || rule.ID != tt.ruleID) {
			ruleID := ""
			if rule != nil {
				ruleID = rule.ID
			}
			t.Errorf("CheckCommand(%q) ruleID=%q, want %q", tt.cmd, ruleID, tt.ruleID)
		}
	}
}

func TestCheckDomain(t *testing.T) {
	e := NewEngine()
	yaml := `
rules:
  - id: block-pastebin
    domain: "pastebin.com"
    action: block
    description: "Block pastebin"
    enabled: true
  - id: block-temp
    domain: "*.temp.sh"
    action: block
    description: "Block temp file hosts"
    enabled: true
  - id: block-xmr
    domain: "*xmr*"
    action: block
    description: "Block XMR domains"
    enabled: true
`
	if err := e.LoadRulesFromBytes([]byte(yaml)); err != nil {
		t.Fatalf("LoadRulesFromBytes: %v", err)
	}

	tests := []struct {
		domain  string
		allowed bool
		ruleID  string
	}{
		{"pastebin.com", false, "block-pastebin"},
		{"PASTEBIN.COM", false, "block-pastebin"},
		{"www.pastebin.com", true, ""}, // Exact match doesn't include subdomains
		{"example.temp.sh", false, "block-temp"},
		{"sub.example.temp.sh", false, "block-temp"},
		{"xmrpool.net", false, "block-xmr"},
		{"pool.xmr.io", false, "block-xmr"},
		{"google.com", true, ""},
	}

	for _, tt := range tests {
		allowed, rule, reason := e.CheckDomain(tt.domain)
		if allowed != tt.allowed {
			t.Errorf("CheckDomain(%q) allowed=%v, want %v (reason: %s)", tt.domain, allowed, tt.allowed, reason)
		}
		if tt.ruleID != "" && (rule == nil || rule.ID != tt.ruleID) {
			ruleID := ""
			if rule != nil {
				ruleID = rule.ID
			}
			t.Errorf("CheckDomain(%q) ruleID=%q, want %q", tt.domain, ruleID, tt.ruleID)
		}
	}
}

func TestLoadDefaultRules(t *testing.T) {
	e := NewEngine()
	err := e.LoadRules("../../pkg/config/default-rules.yaml")
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}

	if e.RuleCount() == 0 {
		t.Error("expected rules to be loaded")
	}

	// Test a known domain rule from default-rules.yaml
	// (command patterns were removed - shield operates at network level only)
	allowed, rule, _ := e.CheckDomain("pastebin.com")
	if allowed {
		t.Error("expected pastebin.com to be blocked")
	}
	if rule == nil || rule.ID != "block-pastebin" {
		t.Errorf("expected rule block-pastebin, got %v", rule)
	}
}

func TestReload(t *testing.T) {
	e := NewEngine()
	err := e.LoadRules("../../pkg/config/default-rules.yaml")
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}

	count := e.RuleCount()
	
	err = e.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}

	if e.RuleCount() != count {
		t.Errorf("rule count changed after reload: %d -> %d", count, e.RuleCount())
	}
}

func TestDefaultAction(t *testing.T) {
	e := NewEngine(WithDefaultAction("block"))
	yaml := `
rules:
  - id: allow-ls
    pattern: "ls *"
    action: allow
    description: "Allow ls"
    enabled: true
`
	if err := e.LoadRulesFromBytes([]byte(yaml)); err != nil {
		t.Fatalf("LoadRulesFromBytes: %v", err)
	}

	// Matched rule allows
	allowed, _, _ := e.CheckCommand("ls -la")
	if !allowed {
		t.Error("expected ls -la to be allowed")
	}

	// Unmatched falls back to block
	allowed, _, reason := e.CheckCommand("cat file.txt")
	if allowed {
		t.Errorf("expected cat to be blocked by default, reason: %s", reason)
	}
}

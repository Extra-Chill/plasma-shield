package rules

import (
	"testing"
)

func TestRuleAppliesToTier(t *testing.T) {
	tests := []struct {
		name     string
		rule     Rule
		tier     string
		expected bool
	}{
		{
			name:     "no tiers specified applies to crew",
			rule:     Rule{Action: "block", Enabled: true},
			tier:     "crew",
			expected: true,
		},
		{
			name:     "no tiers specified - commodore exempt from block",
			rule:     Rule{Action: "block", Enabled: true},
			tier:     "commodore",
			expected: false,
		},
		{
			name:     "no tiers specified - commodore gets allow rules",
			rule:     Rule{Action: "allow", Enabled: true},
			tier:     "commodore",
			expected: true,
		},
		{
			name:     "explicit crew tier matches crew",
			rule:     Rule{Tiers: []string{"crew"}, Action: "block", Enabled: true},
			tier:     "crew",
			expected: true,
		},
		{
			name:     "explicit crew tier does not match captain",
			rule:     Rule{Tiers: []string{"crew"}, Action: "block", Enabled: true},
			tier:     "captain",
			expected: false,
		},
		{
			name:     "crew and captain tier matches captain",
			rule:     Rule{Tiers: []string{"crew", "captain"}, Action: "block", Enabled: true},
			tier:     "captain",
			expected: true,
		},
		{
			name:     "explicit commodore in tiers list applies to commodore",
			rule:     Rule{Tiers: []string{"crew", "commodore"}, Action: "block", Enabled: true},
			tier:     "commodore",
			expected: true,
		},
		{
			name:     "commodore not in explicit list - exempt",
			rule:     Rule{Tiers: []string{"crew", "captain"}, Action: "block", Enabled: true},
			tier:     "commodore",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rule.appliesToTier(tt.tier)
			if got != tt.expected {
				t.Errorf("appliesToTier(%q) = %v, want %v", tt.tier, got, tt.expected)
			}
		})
	}
}

func TestCheckDomainWithTier(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRulesFromBytes([]byte(`
rules:
  - id: block-hetzner
    domain: "api.hetzner.cloud"
    action: block
    tiers: [crew, captain]
    enabled: true
    description: "Block Hetzner API for non-commodore"
  - id: block-pastebin
    domain: "pastebin.com"
    action: block
    enabled: true
    description: "Block pastebin for everyone except commodore"
`))
	if err != nil {
		t.Fatalf("failed to load rules: %v", err)
	}

	tests := []struct {
		name    string
		domain  string
		tier    string
		allowed bool
	}{
		{"crew blocked from hetzner", "api.hetzner.cloud", "crew", false},
		{"captain blocked from hetzner", "api.hetzner.cloud", "captain", false},
		{"commodore allowed hetzner", "api.hetzner.cloud", "commodore", true},
		{"crew blocked from pastebin", "pastebin.com", "crew", false},
		{"commodore exempt from pastebin block", "pastebin.com", "commodore", true},
		{"unknown domain allowed", "google.com", "crew", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, _, _ := engine.CheckDomainWithTier(tt.domain, tt.tier)
			if allowed != tt.allowed {
				t.Errorf("CheckDomainWithTier(%q, %q) = %v, want %v", tt.domain, tt.tier, allowed, tt.allowed)
			}
		})
	}
}

// Package rules provides the rule engine for filtering traffic.
package rules

import (
	"fmt"
	"sync"
)

// Rule defines a single filtering rule.
type Rule struct {
	ID          string   `yaml:"id"`
	Pattern     string   `yaml:"pattern,omitempty"`     // Command pattern to match (glob syntax)
	Domain      string   `yaml:"domain,omitempty"`      // Domain pattern to match
	Action      string   `yaml:"action"`                // "block" or "allow"
	Description string   `yaml:"description,omitempty"` // Human-readable description
	Tiers       []string `yaml:"tiers,omitempty"`       // Tiers this rule applies to (empty = all)
	Enabled     bool     `yaml:"enabled"`
}

// appliesToTier checks if a rule applies to a given tier.
// If rule has no tiers specified, it applies to all tiers.
// Commodore tier is exempt from all restrictive rules by default.
func (r *Rule) appliesToTier(tier string) bool {
	// Commodore is exempt from blocking rules (unless explicitly included)
	if tier == "commodore" && r.Action == "block" {
		// Check if commodore is explicitly in the tiers list
		for _, t := range r.Tiers {
			if t == "commodore" {
				return true
			}
		}
		// Commodore not in list and rule blocks - exempt
		if len(r.Tiers) > 0 {
			return false
		}
		// No tiers specified, commodore still exempt from blocks
		return false
	}

	// If no tiers specified, rule applies to everyone
	if len(r.Tiers) == 0 {
		return true
	}

	// Check if tier is in the list
	for _, t := range r.Tiers {
		if t == tier {
			return true
		}
	}

	return false
}

// RuleSet is a collection of rules.
type RuleSet struct {
	Rules []Rule `yaml:"rules"`
}

// Engine evaluates traffic against rules.
// Thread-safe for concurrent access.
type Engine struct {
	mu            sync.RWMutex
	rules         *RuleSet
	compiled      []*CompiledRule
	rulesPath     string
	defaultAction string // "allow" or "block" when no rules match
}

// EngineOption configures the Engine.
type EngineOption func(*Engine)

// WithDefaultAction sets the default action when no rules match.
// Default is "allow".
func WithDefaultAction(action string) EngineOption {
	return func(e *Engine) {
		e.defaultAction = action
	}
}

// NewEngine creates a new rule engine.
func NewEngine(opts ...EngineOption) *Engine {
	e := &Engine{
		rules:         &RuleSet{},
		compiled:      make([]*CompiledRule, 0),
		defaultAction: "allow",
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// LoadRules loads rules from a YAML file.
// This is the primary method for loading rules.
func (e *Engine) LoadRules(path string) error {
	rs, err := LoadFromFile(path)
	if err != nil {
		return err
	}

	compiled, err := compileRuleSet(rs)
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.rules = rs
	e.compiled = compiled
	e.rulesPath = path
	e.mu.Unlock()

	return nil
}

// LoadRulesFromBytes loads rules from YAML bytes.
// Useful for testing or embedded configurations.
func (e *Engine) LoadRulesFromBytes(data []byte) error {
	rs, err := LoadFromBytes(data)
	if err != nil {
		return err
	}

	compiled, err := compileRuleSet(rs)
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.rules = rs
	e.compiled = compiled
	e.mu.Unlock()

	return nil
}

// Reload reloads rules from the previously loaded file path.
// Returns error if no path was set via LoadRules.
func (e *Engine) Reload() error {
	e.mu.RLock()
	path := e.rulesPath
	e.mu.RUnlock()

	if path == "" {
		return fmt.Errorf("no rules path set; call LoadRules first")
	}

	return e.LoadRules(path)
}

// RulesPath returns the currently loaded rules file path.
func (e *Engine) RulesPath() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.rulesPath
}

// RuleCount returns the number of loaded rules.
func (e *Engine) RuleCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.compiled)
}

// CheckCommand evaluates a command against the ruleset.
// Deprecated: Use CheckCommandWithTier for tier-aware checking.
func (e *Engine) CheckCommand(command string) (allowed bool, matchedRule *Rule, reason string) {
	return e.CheckCommandWithTier(command, "")
}

// CheckCommandWithTier evaluates a command against the ruleset with tier awareness.
// Returns:
//   - allowed: true if the command is allowed, false if blocked
//   - matchedRule: the rule that matched (nil if no match)
//   - reason: human-readable explanation
func (e *Engine) CheckCommandWithTier(command, tier string) (allowed bool, matchedRule *Rule, reason string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, cr := range e.compiled {
		if !cr.Rule.Enabled {
			continue
		}
		if cr.Rule.Pattern == "" {
			continue
		}
		// Skip rules that don't apply to this tier
		if tier != "" && !cr.Rule.appliesToTier(tier) {
			continue
		}
		if cr.MatchCommand(command) {
			if cr.Rule.Action == "block" {
				return false, cr.Rule, fmt.Sprintf("blocked by rule %s: %s", cr.Rule.ID, cr.Rule.Description)
			}
			// Action is "allow" - explicitly allowed
			return true, cr.Rule, fmt.Sprintf("allowed by rule %s: %s", cr.Rule.ID, cr.Rule.Description)
		}
	}

	// No rule matched - use default action
	if e.defaultAction == "block" {
		return false, nil, "blocked by default policy"
	}
	return true, nil, "allowed by default policy"
}

// CheckDomain evaluates a domain against the ruleset.
// Deprecated: Use CheckDomainWithTier for tier-aware checking.
func (e *Engine) CheckDomain(domain string) (allowed bool, matchedRule *Rule, reason string) {
	return e.CheckDomainWithTier(domain, "")
}

// CheckDomainWithTier evaluates a domain against the ruleset with tier awareness.
// Returns:
//   - allowed: true if the domain is allowed, false if blocked
//   - matchedRule: the rule that matched (nil if no match)
//   - reason: human-readable explanation
func (e *Engine) CheckDomainWithTier(domain, tier string) (allowed bool, matchedRule *Rule, reason string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, cr := range e.compiled {
		if !cr.Rule.Enabled {
			continue
		}
		if cr.Rule.Domain == "" {
			continue
		}
		// Skip rules that don't apply to this tier
		if tier != "" && !cr.Rule.appliesToTier(tier) {
			continue
		}
		if cr.MatchDomain(domain) {
			if cr.Rule.Action == "block" {
				return false, cr.Rule, fmt.Sprintf("blocked by rule %s: %s", cr.Rule.ID, cr.Rule.Description)
			}
			// Action is "allow" - explicitly allowed
			return true, cr.Rule, fmt.Sprintf("allowed by rule %s: %s", cr.Rule.ID, cr.Rule.Description)
		}
	}

	// No rule matched - use default action
	if e.defaultAction == "block" {
		return false, nil, "blocked by default policy"
	}
	return true, nil, "allowed by default policy"
}

// compileRuleSet compiles all rules in a RuleSet.
func compileRuleSet(rs *RuleSet) ([]*CompiledRule, error) {
	compiled := make([]*CompiledRule, 0, len(rs.Rules))
	for i := range rs.Rules {
		cr, err := CompileRule(&rs.Rules[i])
		if err != nil {
			return nil, fmt.Errorf("failed to compile rule %s: %w", rs.Rules[i].ID, err)
		}
		compiled = append(compiled, cr)
	}
	return compiled, nil
}

// Package rules provides the rule engine for filtering traffic.
package rules

// Rule defines a single filtering rule.
type Rule struct {
	ID          string `yaml:"id"`
	Pattern     string `yaml:"pattern,omitempty"`     // Command pattern to match
	Domain      string `yaml:"domain,omitempty"`      // Domain pattern to match
	Action      string `yaml:"action"`                // "block" or "allow"
	Description string `yaml:"description,omitempty"` // Human-readable description
	Enabled     bool   `yaml:"enabled"`
}

// RuleSet is a collection of rules.
type RuleSet struct {
	Rules []Rule `yaml:"rules"`
}

// Engine evaluates traffic against rules.
type Engine struct {
	rules *RuleSet
}

// NewEngine creates a new rule engine.
func NewEngine() *Engine {
	return &Engine{
		rules: &RuleSet{},
	}
}

// LoadRules loads rules from a YAML file.
func (e *Engine) LoadRules(path string) error {
	// TODO: Implement YAML loading
	return nil
}

// CheckCommand evaluates a command against the ruleset.
// Returns true if the command is allowed, false if blocked.
func (e *Engine) CheckCommand(command string) (allowed bool, matchedRule *Rule) {
	// TODO: Implement pattern matching
	return true, nil
}

// CheckDomain evaluates a domain against the ruleset.
// Returns true if the domain is allowed, false if blocked.
func (e *Engine) CheckDomain(domain string) (allowed bool, matchedRule *Rule) {
	// TODO: Implement domain matching
	return true, nil
}

# internal/rules

Rule engine for traffic filtering with pattern matching.

## Overview

The rules package provides a thread-safe rule engine that evaluates commands and domains against configurable rules. Rules are defined in YAML and support glob-style patterns for commands and wildcard patterns for domains.

## Types

### Rule

A single filtering rule.

```go
type Rule struct {
    ID          string `yaml:"id"`
    Pattern     string `yaml:"pattern,omitempty"`     // Command pattern (glob)
    Domain      string `yaml:"domain,omitempty"`      // Domain pattern
    Action      string `yaml:"action"`                // "block" or "allow"
    Description string `yaml:"description,omitempty"`
    Enabled     bool   `yaml:"enabled"`
}
```

### RuleSet

Collection of rules, typically loaded from YAML.

```go
type RuleSet struct {
    Rules []Rule `yaml:"rules"`
}
```

### Engine

Thread-safe rule evaluation engine.

```go
type Engine struct {
    rules         *RuleSet
    compiled      []*CompiledRule
    rulesPath     string
    defaultAction string // "allow" or "block"
}
```

### CompiledRule

Pre-compiled rule with regex matchers for efficient evaluation.

```go
type CompiledRule struct {
    Rule           *Rule
    CommandMatcher *regexp.Regexp
    DomainMatcher  *regexp.Regexp
}
```

## Engine Functions

### NewEngine

Creates a new rule engine with optional configuration.

```go
func NewEngine(opts ...EngineOption) *Engine

// Options
func WithDefaultAction(action string) EngineOption
```

Default action is `"allow"` when no rules match.

### Loading Rules

```go
// From file
func (e *Engine) LoadRules(path string) error

// From bytes (for testing or embedded configs)
func (e *Engine) LoadRulesFromBytes(data []byte) error

// Reload from previously loaded path
func (e *Engine) Reload() error
```

### Checking Traffic

```go
// Check a command
func (e *Engine) CheckCommand(command string) (allowed bool, matchedRule *Rule, reason string)

// Check a domain
func (e *Engine) CheckDomain(domain string) (allowed bool, matchedRule *Rule, reason string)
```

### Utility Methods

```go
func (e *Engine) RulesPath() string  // Current rules file path
func (e *Engine) RuleCount() int     // Number of loaded rules
```

## Loader Functions

```go
// Load from file
func LoadFromFile(path string) (*RuleSet, error)

// Parse from bytes
func LoadFromBytes(data []byte) (*RuleSet, error)

// Save to file
func SaveToFile(rs *RuleSet, path string) error
```

## Matcher Functions

### CompileRule

Compiles a rule's patterns into regex matchers.

```go
func CompileRule(r *Rule) (*CompiledRule, error)
```

### CompiledRule Methods

```go
func (cr *CompiledRule) MatchCommand(cmd string) bool
func (cr *CompiledRule) MatchDomain(domain string) bool
```

## Pattern Syntax

### Command Patterns (Glob)

Use `*` as a wildcard that matches any sequence of characters.

| Pattern | Matches | Doesn't Match |
|---------|---------|---------------|
| `rm -rf *` | `rm -rf /`, `rm -rf /tmp` | `rm file.txt` |
| `curl * \| *sh` | `curl url \| bash`, `curl url \| sh` | `curl -o file` |
| `*ssh*` | `ssh host`, `openssh-client` | `scp file` |

Matching is **case-insensitive**.

### Domain Patterns

Three pattern types:

1. **Exact match**: `example.com`
   - Matches only `example.com`
   - Does NOT match `www.example.com`

2. **Wildcard subdomain**: `*.example.com`
   - Matches `sub.example.com`, `a.b.example.com`
   - Also matches base `example.com`

3. **Contains wildcard**: `*xmr*`
   - Matches any domain containing "xmr"
   - Examples: `xmrpool.net`, `pool.xmr.io`

Matching is **case-insensitive**.

## YAML Rule Format

```yaml
rules:
  # Block dangerous commands
  - id: block-rm-rf
    pattern: "rm -rf *"
    action: block
    description: "Block recursive delete"
    enabled: true

  # Block curl pipe to shell
  - id: block-curl-pipe
    pattern: "curl * | *sh"
    action: block
    description: "Block curl pipe to shell"
    enabled: true

  # Block specific domain
  - id: block-pastebin
    domain: "pastebin.com"
    action: block
    description: "Block pastebin"
    enabled: true

  # Block all subdomains
  - id: block-temp-hosts
    domain: "*.temp.sh"
    action: block
    description: "Block temporary file hosts"
    enabled: true

  # Block domains containing pattern
  - id: block-xmr
    domain: "*xmr*"
    action: block
    description: "Block crypto mining domains"
    enabled: true

  # Allow rule (takes precedence if matched first)
  - id: allow-ls
    pattern: "ls *"
    action: allow
    description: "Explicitly allow ls"
    enabled: true
```

## Usage Examples

### Basic Usage

```go
engine := rules.NewEngine()

// Load rules from file
if err := engine.LoadRules("rules.yaml"); err != nil {
    log.Fatal(err)
}

// Check a command
allowed, rule, reason := engine.CheckCommand("rm -rf /tmp")
if !allowed {
    fmt.Printf("Blocked: %s (rule: %s)\n", reason, rule.ID)
}

// Check a domain
allowed, rule, reason = engine.CheckDomain("pastebin.com")
if !allowed {
    fmt.Printf("Blocked: %s (rule: %s)\n", reason, rule.ID)
}
```

### Default Block Policy

```go
// Block everything not explicitly allowed
engine := rules.NewEngine(rules.WithDefaultAction("block"))

err := engine.LoadRulesFromBytes([]byte(`
rules:
  - id: allow-google
    domain: "*.google.com"
    action: allow
    enabled: true
`))

// google.com is allowed
allowed, _, _ := engine.CheckDomain("api.google.com")
// allowed == true

// Everything else is blocked by default
allowed, _, reason := engine.CheckDomain("unknown.com")
// allowed == false, reason == "blocked by default policy"
```

### Hot Reload

```go
engine := rules.NewEngine()
engine.LoadRules("/etc/plasma-shield/rules.yaml")

// Later, reload rules without restart
if err := engine.Reload(); err != nil {
    log.Printf("Failed to reload rules: %v", err)
}
```

## Testing Examples

From `engine_test.go`:

```go
func TestCheckCommand(t *testing.T) {
    e := NewEngine()
    yaml := `
rules:
  - id: block-rm-rf
    pattern: "rm -rf *"
    action: block
    description: "Block recursive delete"
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
        {"rm file.txt", true, ""},
        {"ls -la", true, ""},
    }

    for _, tt := range tests {
        allowed, rule, _ := e.CheckCommand(tt.cmd)
        if allowed != tt.allowed {
            t.Errorf("CheckCommand(%q) = %v, want %v", tt.cmd, allowed, tt.allowed)
        }
    }
}
```

## Thread Safety

The Engine is thread-safe for concurrent access:
- Rule loading acquires a write lock
- Rule checking acquires a read lock
- Multiple goroutines can check rules simultaneously

## Rule Evaluation Order

1. Rules are evaluated in definition order
2. First matching rule determines the outcome
3. If no rule matches, default action is used (allow or block)

## Package Dependencies

```
rules
├── regexp      - Pattern compilation
├── sync        - Thread safety
├── gopkg.in/yaml.v3  - YAML parsing
└── os          - File I/O
```

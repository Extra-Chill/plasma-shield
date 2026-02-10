# internal/mode

Operating mode management for the shield router.

## Overview

The mode package controls how Plasma Shield responds to rule matches. It supports global modes and per-agent overrides, enabling scenarios like testing new rules (audit mode) or emergency response (lockdown mode).

## Mode Types

```go
type Mode string

const (
    Enforce  Mode = "enforce"   // Normal operation - block matching requests
    Audit    Mode = "audit"     // Log everything, never block (testing)
    Lockdown Mode = "lockdown"  // Block ALL requests (emergency)
)
```

## Mode Behavior Matrix

| Mode | Rule Matched | Result |
|------|--------------|--------|
| **Enforce** | Yes | Block |
| **Enforce** | No | Allow |
| **Audit** | Yes | Allow (log "would block") |
| **Audit** | No | Allow |
| **Lockdown** | Yes | Block |
| **Lockdown** | No | Block |

## Manager Type

Thread-safe mode management with global and per-agent settings.

```go
type Manager struct {
    globalMode  Mode
    agentModes  map[string]Mode  // agent ID -> mode override
}
```

## Functions

### NewManager

Creates a new mode manager with `Enforce` as the default global mode.

```go
func NewManager() *Manager
```

### Global Mode

```go
// Get current global mode
func (m *Manager) GlobalMode() Mode

// Set global mode
func (m *Manager) SetGlobalMode(mode Mode)
```

### Per-Agent Mode

```go
// Get effective mode for an agent (override or global)
func (m *Manager) AgentMode(agentID string) Mode

// Set mode override for specific agent
func (m *Manager) SetAgentMode(agentID string, mode Mode)

// Remove agent override (reverts to global)
func (m *Manager) ClearAgentMode(agentID string)

// Get all agent overrides (returns a copy)
func (m *Manager) AllAgentModes() map[string]Mode
```

### Decision Helpers

```go
// Determine if a request should be blocked
func (m *Manager) ShouldBlock(agentID string, ruleMatched bool) bool

// Check if agent is in audit mode
func (m *Manager) IsAudit(agentID string) bool
```

## Usage Examples

### Basic Mode Management

```go
m := mode.NewManager()

// Default is Enforce
fmt.Println(m.GlobalMode()) // "enforce"

// Switch to audit mode globally
m.SetGlobalMode(mode.Audit)

// Check if request should be blocked
shouldBlock := m.ShouldBlock("agent-1", true) // false (audit mode)
```

### Per-Agent Overrides

```go
m := mode.NewManager()
m.SetGlobalMode(mode.Enforce)

// Put one agent in audit mode for testing
m.SetAgentMode("test-agent", mode.Audit)

// test-agent won't block anything
m.ShouldBlock("test-agent", true) // false

// Other agents still enforce
m.ShouldBlock("prod-agent", true) // true
```

### Emergency Lockdown

```go
m := mode.NewManager()

// Lockdown a specific compromised agent
m.SetAgentMode("compromised-agent", mode.Lockdown)

// All requests blocked regardless of rules
m.ShouldBlock("compromised-agent", false) // true

// Or global lockdown
m.SetGlobalMode(mode.Lockdown)
```

### Testing New Rules

```go
m := mode.NewManager()

// Put system in audit mode to test new rules
m.SetGlobalMode(mode.Audit)

// Deploy new rules...
// Monitor logs for "would have blocked" entries

// If rules look good, enable enforcement
m.SetGlobalMode(mode.Enforce)
```

## Integration with Proxy

The proxy `Inspector` uses mode manager to decide blocking:

```go
// From proxy/inspector.go
func (i *Inspector) CheckRequest(r *http.Request) (shouldBlock bool, ruleMatched bool, reason string) {
    agentID := i.ExtractAgentToken(r)
    host := i.ExtractHost(r)

    // Check rules
    allowed, matchedRule, ruleReason := i.engine.CheckDomain(host)
    ruleMatched = !allowed

    // Mode-aware blocking decision
    shouldBlock = i.modeManager.ShouldBlock(agentID, ruleMatched)

    // ...logging...
    return shouldBlock, ruleMatched, reason
}
```

## Thread Safety

The Manager is fully thread-safe:
- All methods use read/write locks appropriately
- Safe for concurrent access from multiple goroutines
- `AllAgentModes()` returns a copy to prevent race conditions

## Testing Examples

From `mode_test.go`:

```go
func TestShouldBlock(t *testing.T) {
    tests := []struct {
        name        string
        mode        Mode
        ruleMatched bool
        want        bool
    }{
        {"enforce with rule matched", Enforce, true, true},
        {"enforce with rule not matched", Enforce, false, false},
        {"audit with rule matched", Audit, true, false},
        {"audit with rule not matched", Audit, false, false},
        {"lockdown with rule matched", Lockdown, true, true},
        {"lockdown with rule not matched", Lockdown, false, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := NewManager()
            agentID := "test-agent"
            m.SetAgentMode(agentID, tt.mode)

            got := m.ShouldBlock(agentID, tt.ruleMatched)
            if got != tt.want {
                t.Errorf("ShouldBlock() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Concurrent Access Test

```go
func TestConcurrentAccess(t *testing.T) {
    m := NewManager()
    const numGoroutines = 100

    var wg sync.WaitGroup
    wg.Add(numGoroutines)

    for i := 0; i < numGoroutines; i++ {
        go func(id int) {
            defer wg.Done()
            agentID := "agent"

            // Mix of operations
            m.SetGlobalMode(Audit)
            m.GlobalMode()
            m.SetAgentMode(agentID, Lockdown)
            m.AgentMode(agentID)
            m.ShouldBlock(agentID, true)
            m.ClearAgentMode(agentID)
        }(i)
    }

    wg.Wait()
    // Test passes if no race conditions
}
```

## Common Patterns

### Gradual Rollout

1. Start with audit mode globally
2. Enable enforce for one agent at a time
3. Monitor for issues
4. Enable globally when confident

```go
// Start with audit
m.SetGlobalMode(mode.Audit)

// Enable enforce for canary agent
m.SetAgentMode("canary-agent", mode.Enforce)

// ... wait and monitor ...

// Roll out to more agents
for _, agentID := range readyAgents {
    m.SetAgentMode(agentID, mode.Enforce)
}

// Finally, global enforce
m.SetGlobalMode(mode.Enforce)
for _, agentID := range allAgents {
    m.ClearAgentMode(agentID)
}
```

### Incident Response

```go
// Suspicious activity detected on agent
m.SetAgentMode("suspicious-agent", mode.Lockdown)

// Log the action
log.Printf("Agent %s put in lockdown due to suspicious activity", agentID)

// Investigate...

// If cleared, resume normal operation
m.ClearAgentMode("suspicious-agent")
```

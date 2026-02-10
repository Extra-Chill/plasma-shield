# SSH Bastion Service - Implementation Plan

## Overview

Add SSH bastion functionality to Plasma Shield, enabling operators and users to debug agents without storing SSH keys on the shield or giving agents lateral SSH access.

## Phases

### Phase 1: Core SSH Server
**Goal:** Basic SSH jump host that proxies connections to target agents.

**Deliverables:**
- `internal/bastion/server.go` — SSH server using `golang.org/x/crypto/ssh`
- `internal/bastion/proxy.go` — Connection proxying to target hosts
- Host key generation and storage
- Integration with `cmd/proxy/main.go` (new `-bastion-addr` flag)

**Acceptance:**
```bash
# Start proxy with bastion
./plasma-shield-router -bastion-addr :2222

# SSH through bastion (with pre-shared key for now)
ssh -J localhost:2222 root@target-agent
```

**Estimate:** ~200 lines

---

### Phase 2: Access Grant System
**Goal:** Time-limited, logged access grants that control who can SSH where.

**Deliverables:**
- `internal/bastion/grants.go` — Grant storage (in-memory + JSON persistence)
- `internal/bastion/types.go` — Grant struct with expiration, scope, audit fields
- API endpoints: `POST /grants`, `GET /grants`, `DELETE /grants/{id}`
- CLI commands: `plasma-shield access grant`, `access list`, `access revoke`

**Grant structure:**
```go
type Grant struct {
    ID        string    `json:"id"`
    Principal string    `json:"principal"`  // who can use this grant
    Target    string    `json:"target"`     // agent or fleet pattern
    ExpiresAt time.Time `json:"expires_at"`
    CreatedBy string    `json:"created_by"` // audit trail
    CreatedAt time.Time `json:"created_at"`
}
```

**Acceptance:**
```bash
# Create grant
plasma-shield access grant --target sarai-chinwag --duration 30m

# List active grants
plasma-shield access list

# Revoke
plasma-shield access revoke <grant-id>
```

**Estimate:** ~250 lines

---

### Phase 3: Certificate Authority
**Goal:** Issue short-lived SSH certificates instead of managing keys.

**Deliverables:**
- `internal/bastion/ca.go` — Certificate authority (signing, validation)
- CA keypair generation and secure storage
- Certificate issuance tied to grants
- Client certificate validation on SSH connection

**Flow:**
1. User requests access → grant created
2. User authenticates to bastion (their existing key or token)
3. Bastion issues short-lived cert (matches grant duration)
4. Cert used for actual SSH to target

**Acceptance:**
```bash
# User gets cert automatically when connecting through bastion
ssh -J bastion:2222 target-agent
# Cert expires with grant, no long-lived credentials
```

**Estimate:** ~200 lines

---

### Phase 4: Session Logging
**Goal:** Full audit trail of SSH sessions for compliance and debugging.

**Deliverables:**
- `internal/bastion/logger.go` — Session event logging
- Log connection start/end, target, duration
- Optional command logging (PTY capture)
- API endpoint: `GET /bastion/sessions`
- Integration with existing LogStore pattern

**Log events:**
```go
type SessionEvent struct {
    SessionID string    `json:"session_id"`
    GrantID   string    `json:"grant_id"`
    Principal string    `json:"principal"`
    Target    string    `json:"target"`
    Event     string    `json:"event"`  // connect, disconnect, command
    Timestamp time.Time `json:"timestamp"`
    Data      string    `json:"data,omitempty"`  // command text if captured
}
```

**Estimate:** ~150 lines

---

### Phase 5: Admin Panel Integration
**Goal:** Web UI for managing access grants and viewing sessions.

**Deliverables:**
- Grants section in dashboard (create, view, revoke)
- Session history view
- Active connections indicator
- Grant expiration countdown

**UI Components:**
- Grant creation form (target, duration)
- Active grants table with revoke buttons
- Session log viewer with filters

**Estimate:** ~300 lines (HTML/JS in embedded UI)

---

## Dependencies

```
golang.org/x/crypto/ssh  — SSH server and client
```

## File Structure

```
internal/bastion/
├── server.go      # SSH server
├── proxy.go       # Connection proxying
├── grants.go      # Access grant storage
├── ca.go          # Certificate authority
├── logger.go      # Session logging
└── types.go       # Shared types

cmd/proxy/main.go  # Add -bastion-addr flag
```

## Security Considerations

- CA private key must be protected (file permissions, no agent access)
- Grants scoped to specific targets, no wildcards without explicit opt-in
- All sessions logged regardless of grant type
- Bastion binds to configurable interface (default: all, for jump host use)
- Admin API still localhost-only

## Testing Strategy

Each phase includes tests:
- Phase 1: Connection proxying works
- Phase 2: Grant expiration enforced, persistence survives restart
- Phase 3: Invalid/expired certs rejected
- Phase 4: All events captured
- Phase 5: UI functional tests (manual)

## Rollout

1. Phases 1-2: Functional bastion with grant system
2. Phase 3: Production-ready security (certs)
3. Phases 4-5: Operational polish

Phases 1-2 can ship as "beta" for internal use. Phase 3 required for external/SaaS deployment.

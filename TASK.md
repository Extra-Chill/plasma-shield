# Task: Container-Aware Routing (Issue #1)

## Context
Plasma Shield currently identifies agents by IP address (1 agent = 1 IP). We're moving to multi-tenant containers where multiple agents share one host IP. The shield needs to identify agents by container, not just IP.

## What to Change

### 1. Agent struct (`internal/fleet/fleet.go`)
Add container fields to Agent:
```go
type Agent struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    IP          string `json:"ip,omitempty"`
    ContainerID string `json:"container_id,omitempty"`  // Docker container ID/name
    SourcePort  int    `json:"source_port,omitempty"`   // If using port-based routing
    Tier        string `json:"tier,omitempty"`
    Description string `json:"description,omitempty"`
    WebhookURL  string `json:"webhook_url,omitempty"`
    HostID      string `json:"host_id,omitempty"`       // Which host this container is on
}
```

### 2. Manager (`internal/fleet/fleet.go`)
- Add `containerToAgent map[string]*Agent` for container-based lookups
- Add `ValidateAgentContainer(hostIP, containerID string) (agentID, tier string, valid bool)`
- Update `AddAgent` to index by container when ContainerID is set
- Keep IP-based lookup as fallback for non-containerized agents (backward compat)

### 3. Config (`internal/fleet/loader.go`)
Support the new host-based YAML format:
```yaml
tenants:
  - id: chubes
    captain_name: "Chubes"
    mode: fleet
    hosts:
      - id: sarai-chinwag
        ip: 178.156.229.129
        agents:
          - id: sarai
            name: "Sarai Chinwag"
            container_id: sarai-wp
            tier: crew
          - id: carrieann
            name: "Carrie-Ann"
            container_id: carrieann-wp
            tier: customer
    agents:  # Legacy format still works
      - id: command
        name: "Star Fleet Command"
        ip: 178.156.153.244
        tier: commodore
```

### 4. Proxy Handler (`internal/proxy/handler.go`)
- Check for `X-Container-ID` header (set by host nginx/Docker network)
- If present: validate via container lookup
- If absent: fall back to IP-based validation
- Add `ContainerID` to LogEntry

### 5. Tests
- Update existing tests to still pass (backward compat)
- Add new tests for:
  - Container-based agent validation
  - Multiple agents on same host IP
  - Mixed config (some agents IP-based, some container-based)
  - Host config format parsing
  - Log entries include container metadata

## Constraints
- MUST be backward compatible — existing IP-only configs still work
- All existing tests must pass
- Run `go test ./...` before committing

# Plasma Shield Configuration

Plasma Shield uses YAML rule files to define network-level blocking policies for AI agents.

> **Important:** Plasma Shield operates at the **network level**. It blocks domains, URLs, and request patterns. It cannot block local exec commands — use OS-level controls (limited user permissions, containers, seccomp, AppArmor) for that.

## Rule File Format

Rules are defined in YAML format under a top-level `rules:` key:

```yaml
rules:
  - id: rule-identifier
    domain: "example.com"
    action: block
    description: "Why this rule exists"
    enabled: true
```

## Rule Options

Each rule supports the following fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique identifier for the rule |
| `domain` | string | No* | Domain pattern to match |
| `url_pattern` | string | No* | URL pattern to match (for HTTP inspection) |
| `action` | string | Yes | Action to take (`block`) |
| `description` | string | No | Human-readable explanation |
| `enabled` | boolean | No | Whether the rule is active (default: true) |

*Either `domain` or `url_pattern` must be specified.

### Domain Matching

Domain patterns support wildcards:

- `example.com` — exact match
- `*.example.com` — matches subdomains (e.g., `sub.example.com`)
- `*keyword*` — matches domains containing the keyword

### URL Pattern Matching

URL patterns match against the full request URL:

- `*env=*AWS_*` — matches URLs with AWS credentials in query params
- `*api_key=*` — matches URLs containing `api_key=`

## Default Rules

Plasma Shield ships with rules covering common threat categories:

### Dangerous Domains

| ID | Domain | Description |
|----|--------|-------------|
| `block-pastebin` | `pastebin.com` | Block pastebin (common for malware hosting) |
| `block-temp-file-hosts` | `*.temp.sh` | Block temporary file hosting services |
| `block-transfer-sh` | `transfer.sh` | Block file transfer service |
| `block-0x0` | `0x0.st` | Block anonymous file hosting |

### Cryptocurrency Mining

| ID | Domain | Description |
|----|--------|-------------|
| `block-crypto-pools` | `*pool.com` | Block common mining pool domains |
| `block-xmr-mining` | `*xmr*` | Block Monero mining domains |
| `block-nicehash` | `*.nicehash.com` | Block NiceHash mining |

### C2 / Exfiltration Tunnels

| ID | Domain | Description |
|----|--------|-------------|
| `block-ngrok` | `*.ngrok.io` | Block ngrok tunnels (common for C2) |
| `block-serveo` | `serveo.net` | Block serveo tunnels |
| `block-localtunnel` | `*.loca.lt` | Block localtunnel |

### Request Pattern Inspection

| ID | Pattern | Description |
|----|---------|-------------|
| `block-sensitive-env-exfil` | `*env=*AWS_*` | Block URLs containing AWS credentials in query params |
| `block-api-key-exfil` | `*api_key=*` | Block URLs with api_key in query params |

## Adding Custom Rules

Create a custom rules file (e.g., `custom-rules.yaml`) following the same format:

```yaml
rules:
  # Block a specific domain
  - id: block-my-domain
    domain: "untrusted-site.com"
    action: block
    description: "Block untrusted external site"
    enabled: true

  # Block all subdomains of a service
  - id: block-risky-service
    domain: "*.risky-service.io"
    action: block
    description: "Block risky cloud service"
    enabled: true

  # Block URL patterns (HTTP inspection)
  - id: block-token-exfil
    url_pattern: "*token=*"
    action: block
    description: "Prevent token leakage via URL params"
    enabled: true

  # Disable a rule temporarily
  - id: temporarily-disabled
    domain: "example.com"
    action: block
    description: "Currently disabled for testing"
    enabled: false
```

### Best Practices

1. **Use descriptive IDs** — Makes logs and debugging easier
2. **Always add descriptions** — Document why each rule exists
3. **Test wildcards carefully** — `*pool.com` will match `carpool.com` too
4. **Use `enabled: false`** — Disable rules temporarily instead of deleting them

## Limitations

Plasma Shield is a **network-level** filter. It cannot protect against:

- Local exec commands (`rm -rf /`, `curl | bash`)
- File system operations
- Process spawning

For exec hardening, use OS-level controls:

1. Run agent as limited user (no root)
2. Use Docker with `--cap-drop=ALL --read-only`
3. Apply seccomp profiles to restrict syscalls
4. Use AppArmor/SELinux for mandatory access control
5. Restrict PATH to safe commands only

See `docs/exec-hardening.md` for detailed guidance.

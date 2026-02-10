// Plasma Shield CLI
//
// Human-only management interface for the shield router.
// Install on your personal machine, not on agent VPSes.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var version = "0.1.0"

// Global config flags
var (
	apiURL    string
	authToken string
	jsonOut   bool
)

func main() {
	// Parse global flags
	flag.StringVar(&apiURL, "api-url", getEnvOrDefault("PLASMA_API_URL", "http://localhost:8443"), "Shield API URL")
	flag.StringVar(&authToken, "token", os.Getenv("PLASMA_TOKEN"), "Bearer auth token")
	flag.BoolVar(&jsonOut, "json", false, "Output JSON for machine parsing")

	// Custom usage to show subcommands
	flag.Usage = printUsage

	// Parse flags but stop at first non-flag (subcommand)
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		os.Exit(0)
	}

	switch args[0] {
	case "version", "--version", "-v":
		if jsonOut {
			outputJSON(map[string]string{"version": version})
		} else {
			fmt.Printf("plasma-shield v%s\n", version)
		}
	case "status":
		handleStatus()
	case "mode":
		if len(args) < 2 {
			fmt.Println("Usage: plasma-shield mode <enforce|audit|lockdown>")
			fmt.Println("\nModes:")
			fmt.Println("  enforce   Normal operation - block matching requests (default)")
			fmt.Println("  audit     Log all requests but never block (testing)")
			fmt.Println("  lockdown  Block ALL outbound requests (emergency)")
			os.Exit(1)
		}
		handleMode(args[1:])
	case "agent":
		if len(args) < 2 {
			fmt.Println("Usage: plasma-shield agent <list|pause|kill|resume> [agent-id]")
			os.Exit(1)
		}
		handleAgent(args[1:])
	case "rules":
		if len(args) < 2 {
			fmt.Println("Usage: plasma-shield rules <list|add|remove> [options]")
			os.Exit(1)
		}
		handleRules(args[1:])
	case "logs":
		handleLogs(args[1:])
	case "auth":
		if len(args) < 2 {
			fmt.Println("Usage: plasma-shield auth <login|logout>")
			os.Exit(1)
		}
		handleAuth(args[1:])
	case "access":
		if len(args) < 2 {
			fmt.Println("Usage: plasma-shield access <grant|list|revoke> [options]")
			os.Exit(1)
		}
		handleAccess(args[1:])
	default:
		fmt.Printf("Unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func printUsage() {
	fmt.Println(`Plasma Shield CLI - Network security for AI agent fleets

Usage: plasma-shield [global-flags] <command> [options]

Global Flags:
  --api-url string   Shield API URL (default: http://localhost:8443, env: PLASMA_API_URL)
  --token string     Bearer auth token (env: PLASMA_TOKEN)
  --json             Output JSON for machine parsing

Commands:
  status          Show shield connection status
  mode            Set global operating mode (enforce/audit/lockdown)
  agent           Manage agents (list, pause, kill, resume)
  rules           Manage blocking rules
  access          Manage SSH bastion access grants
  logs            View traffic logs
  auth            Authentication (login, logout)
  version         Show version

Modes:
  enforce         Normal operation - block matching requests (default)
  audit           Log everything but never block (testing/debugging)
  lockdown        Block ALL outbound requests (emergency)

Examples:
  plasma-shield status
  plasma-shield mode audit                    # Enable audit mode globally
  plasma-shield mode enforce                  # Back to normal
  plasma-shield agent list
  plasma-shield agent pause sarai             # Pause an agent
  plasma-shield agent kill sarai              # Emergency stop
  plasma-shield agent resume sarai            # Resume a paused agent
  plasma-shield rules list
  plasma-shield rules add --pattern "rm -rf" --action block
  plasma-shield rules remove <rule-id>
  plasma-shield access grant --target sarai-chinwag --duration 30m
  plasma-shield access list
  plasma-shield access revoke <grant-id>
  plasma-shield logs --limit 50 --agent sarai

Documentation: https://github.com/Extra-Chill/plasma-shield`)
}

// API client helpers

func apiRequest(method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	url := strings.TrimSuffix(apiURL, "/") + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

func outputJSON(data interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

func exitError(msg string, code int) {
	if jsonOut {
		outputJSON(map[string]interface{}{
			"error": msg,
			"code":  code,
		})
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	}
	os.Exit(code)
}

// Command handlers

func handleStatus() {
	respBody, statusCode, err := apiRequest("GET", "/status", nil)
	if err != nil {
		exitError(err.Error(), 1)
	}

	if statusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.Unmarshal(respBody, &errResp)
		exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
	}

	var status struct {
		Status        string    `json:"status"`
		Version       string    `json:"version"`
		Uptime        string    `json:"uptime"`
		StartedAt     time.Time `json:"started_at"`
		AgentCount    int       `json:"agent_count"`
		RuleCount     int       `json:"rule_count"`
		RequestsTotal int64     `json:"requests_total"`
		BlockedTotal  int64     `json:"blocked_total"`
	}

	if err := json.Unmarshal(respBody, &status); err != nil {
		exitError(fmt.Sprintf("failed to parse response: %v", err), 1)
	}

	if jsonOut {
		outputJSON(status)
	} else {
		fmt.Printf("Shield Status: %s\n", status.Status)
		fmt.Printf("Version: %s\n", status.Version)
		fmt.Printf("Uptime: %s\n", status.Uptime)
		fmt.Printf("Agents: %d\n", status.AgentCount)
		fmt.Printf("Rules: %d\n", status.RuleCount)
		fmt.Printf("Total Requests: %d\n", status.RequestsTotal)
		fmt.Printf("Total Blocked: %d\n", status.BlockedTotal)
	}
}

func handleAgent(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: plasma-shield agent <list|pause|kill|resume> [agent-id]")
		os.Exit(1)
	}

	action := args[0]

	switch action {
	case "list":
		respBody, statusCode, err := apiRequest("GET", "/agents", nil)
		if err != nil {
			exitError(err.Error(), 1)
		}

		if statusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(respBody, &errResp)
			exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
		}

		var response struct {
			Agents []struct {
				ID        string    `json:"id"`
				Name      string    `json:"name"`
				IP        string    `json:"ip"`
				Status    string    `json:"status"`
				LastSeen  time.Time `json:"last_seen"`
				CreatedAt time.Time `json:"created_at"`
			} `json:"agents"`
			Total int `json:"total"`
		}

		if err := json.Unmarshal(respBody, &response); err != nil {
			exitError(fmt.Sprintf("failed to parse response: %v", err), 1)
		}

		if jsonOut {
			outputJSON(response)
		} else {
			if response.Total == 0 {
				fmt.Println("No agents registered")
			} else {
				fmt.Printf("Agents (%d total):\n", response.Total)
				fmt.Println("─────────────────────────────────────────────────────────────")
				for _, agent := range response.Agents {
					statusIcon := "●"
					switch agent.Status {
					case "active":
						statusIcon = "✓"
					case "paused":
						statusIcon = "⏸"
					case "killed":
						statusIcon = "✗"
					}
					fmt.Printf("%s %-12s %-15s %-8s (last seen: %s)\n",
						statusIcon, agent.Name, agent.IP, agent.Status,
						agent.LastSeen.Format("2006-01-02 15:04:05"))
				}
			}
		}

	case "pause":
		if len(args) < 2 {
			exitError("agent ID required: plasma-shield agent pause <agent-id>", 1)
		}
		agentID := args[1]

		respBody, statusCode, err := apiRequest("POST", "/agents/"+agentID+"/pause", nil)
		if err != nil {
			exitError(err.Error(), 1)
		}

		if statusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(respBody, &errResp)
			exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
		}

		var response struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		json.Unmarshal(respBody, &response)

		if jsonOut {
			outputJSON(response)
		} else {
			fmt.Printf("✓ %s\n", response.Message)
		}

	case "kill":
		if len(args) < 2 {
			exitError("agent ID required: plasma-shield agent kill <agent-id>", 1)
		}
		agentID := args[1]

		respBody, statusCode, err := apiRequest("POST", "/agents/"+agentID+"/kill", nil)
		if err != nil {
			exitError(err.Error(), 1)
		}

		if statusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(respBody, &errResp)
			exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
		}

		var response struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		json.Unmarshal(respBody, &response)

		if jsonOut {
			outputJSON(response)
		} else {
			fmt.Printf("⚠ %s\n", response.Message)
		}

	case "resume":
		if len(args) < 2 {
			exitError("agent ID required: plasma-shield agent resume <agent-id>", 1)
		}
		agentID := args[1]

		respBody, statusCode, err := apiRequest("POST", "/agents/"+agentID+"/resume", nil)
		if err != nil {
			exitError(err.Error(), 1)
		}

		if statusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(respBody, &errResp)
			exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
		}

		var response struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		json.Unmarshal(respBody, &response)

		if jsonOut {
			outputJSON(response)
		} else {
			fmt.Printf("✓ %s\n", response.Message)
		}

	default:
		exitError(fmt.Sprintf("unknown agent action: %s (use list, pause, kill, or resume)", action), 1)
	}
}

func handleRules(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: plasma-shield rules <list|add|remove> [options]")
		os.Exit(1)
	}

	action := args[0]

	switch action {
	case "list":
		respBody, statusCode, err := apiRequest("GET", "/rules", nil)
		if err != nil {
			exitError(err.Error(), 1)
		}

		if statusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(respBody, &errResp)
			exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
		}

		var response struct {
			Rules []struct {
				ID          string    `json:"id"`
				Pattern     string    `json:"pattern,omitempty"`
				Domain      string    `json:"domain,omitempty"`
				Action      string    `json:"action"`
				Description string    `json:"description,omitempty"`
				Enabled     bool      `json:"enabled"`
				CreatedAt   time.Time `json:"created_at"`
			} `json:"rules"`
			Total int `json:"total"`
		}

		if err := json.Unmarshal(respBody, &response); err != nil {
			exitError(fmt.Sprintf("failed to parse response: %v", err), 1)
		}

		if jsonOut {
			outputJSON(response)
		} else {
			if response.Total == 0 {
				fmt.Println("No rules configured")
			} else {
				fmt.Printf("Rules (%d total):\n", response.Total)
				fmt.Println("─────────────────────────────────────────────────────────────")
				for _, rule := range response.Rules {
					enabledIcon := "✓"
					if !rule.Enabled {
						enabledIcon = "○"
					}
					target := rule.Pattern
					if target == "" {
						target = rule.Domain
					}
					fmt.Printf("%s [%s] %-6s %s\n", enabledIcon, rule.ID[:8], rule.Action, target)
					if rule.Description != "" {
						fmt.Printf("          %s\n", rule.Description)
					}
				}
			}
		}

	case "add":
		// Parse add flags
		addFlags := flag.NewFlagSet("rules add", flag.ExitOnError)
		pattern := addFlags.String("pattern", "", "Command pattern to match")
		domain := addFlags.String("domain", "", "Domain to match")
		ruleAction := addFlags.String("action", "block", "Action: block or allow")
		description := addFlags.String("desc", "", "Rule description")
		enabled := addFlags.Bool("enabled", true, "Enable the rule")

		addFlags.Parse(args[1:])

		if *pattern == "" && *domain == "" {
			exitError("either --pattern or --domain is required", 1)
		}

		reqBody := map[string]interface{}{
			"pattern":     *pattern,
			"domain":      *domain,
			"action":      *ruleAction,
			"description": *description,
			"enabled":     *enabled,
		}

		respBody, statusCode, err := apiRequest("POST", "/rules", reqBody)
		if err != nil {
			exitError(err.Error(), 1)
		}

		if statusCode != http.StatusCreated && statusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(respBody, &errResp)
			exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
		}

		var response struct {
			Rule struct {
				ID          string    `json:"id"`
				Pattern     string    `json:"pattern,omitempty"`
				Domain      string    `json:"domain,omitempty"`
				Action      string    `json:"action"`
				Description string    `json:"description,omitempty"`
				Enabled     bool      `json:"enabled"`
				CreatedAt   time.Time `json:"created_at"`
			} `json:"rule"`
			Message string `json:"message"`
		}
		json.Unmarshal(respBody, &response)

		if jsonOut {
			outputJSON(response)
		} else {
			fmt.Printf("✓ Rule created: %s\n", response.Rule.ID)
			target := response.Rule.Pattern
			if target == "" {
				target = response.Rule.Domain
			}
			fmt.Printf("  Action: %s\n", response.Rule.Action)
			fmt.Printf("  Target: %s\n", target)
		}

	case "remove":
		if len(args) < 2 {
			exitError("rule ID required: plasma-shield rules remove <rule-id>", 1)
		}
		ruleID := args[1]

		respBody, statusCode, err := apiRequest("DELETE", "/rules/"+ruleID, nil)
		if err != nil {
			exitError(err.Error(), 1)
		}

		if statusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(respBody, &errResp)
			exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
		}

		var response struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		}
		json.Unmarshal(respBody, &response)

		if jsonOut {
			outputJSON(response)
		} else {
			fmt.Printf("✓ %s\n", response.Message)
		}

	default:
		exitError(fmt.Sprintf("unknown rules action: %s (use list, add, or remove)", action), 1)
	}
}

func handleLogs(args []string) {
	// Parse logs flags
	logsFlags := flag.NewFlagSet("logs", flag.ExitOnError)
	limit := logsFlags.Int("limit", 100, "Number of logs to return")
	offset := logsFlags.Int("offset", 0, "Offset for pagination")
	agentID := logsFlags.String("agent", "", "Filter by agent ID")
	actionFilter := logsFlags.String("action", "", "Filter by action (allowed/blocked)")
	typeFilter := logsFlags.String("type", "", "Filter by type (command/http/dns)")

	logsFlags.Parse(args)

	// Build query string
	query := fmt.Sprintf("?limit=%d&offset=%d", *limit, *offset)
	if *agentID != "" {
		query += "&agent_id=" + *agentID
	}
	if *actionFilter != "" {
		query += "&action=" + *actionFilter
	}
	if *typeFilter != "" {
		query += "&type=" + *typeFilter
	}

	respBody, statusCode, err := apiRequest("GET", "/logs"+query, nil)
	if err != nil {
		exitError(err.Error(), 1)
	}

	if statusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.Unmarshal(respBody, &errResp)
		exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
	}

	var response struct {
		Logs []struct {
			ID        string    `json:"id"`
			Timestamp time.Time `json:"timestamp"`
			AgentID   string    `json:"agent_id"`
			Type      string    `json:"type"`
			Request   string    `json:"request"`
			Action    string    `json:"action"`
			RuleID    string    `json:"rule_id,omitempty"`
		} `json:"logs"`
		Total  int `json:"total"`
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		exitError(fmt.Sprintf("failed to parse response: %v", err), 1)
	}

	if jsonOut {
		outputJSON(response)
	} else {
		if len(response.Logs) == 0 {
			fmt.Println("No logs found")
		} else {
			fmt.Printf("Logs (showing %d of %d):\n", len(response.Logs), response.Total)
			fmt.Println("─────────────────────────────────────────────────────────────")
			for _, log := range response.Logs {
				actionIcon := "✓"
				if log.Action == "blocked" {
					actionIcon = "✗"
				}
				timestamp := log.Timestamp.Format("15:04:05")
				request := log.Request
				if len(request) > 50 {
					request = request[:47] + "..."
				}
				fmt.Printf("%s %s %-10s %-8s %s\n",
					actionIcon, timestamp, log.AgentID, log.Action, request)
			}
		}
	}
}

func handleMode(args []string) {
	if len(args) < 1 {
		exitError("mode required: enforce, audit, or lockdown", 1)
	}

	mode := args[0]
	switch mode {
	case "enforce", "audit", "lockdown":
		// Try to call PUT /mode
		reqBody := map[string]string{"mode": mode}
		respBody, statusCode, err := apiRequest("PUT", "/mode", reqBody)
		if err != nil {
			exitError(err.Error(), 1)
		}

		if statusCode == http.StatusNotFound {
			// Endpoint not implemented yet
			if jsonOut {
				outputJSON(map[string]interface{}{
					"error":   "mode endpoint not yet implemented in API",
					"message": "global mode management coming soon",
				})
			} else {
				fmt.Printf("Note: Global mode endpoint not yet implemented in API\n")
				fmt.Printf("Requested mode: %s\n", mode)
			}
			return
		}

		if statusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(respBody, &errResp)
			exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
		}

		var response struct {
			Mode    string `json:"mode"`
			Message string `json:"message"`
		}
		json.Unmarshal(respBody, &response)

		if jsonOut {
			outputJSON(response)
		} else {
			fmt.Printf("✓ Global mode set to: %s\n", mode)
		}

	default:
		exitError(fmt.Sprintf("unknown mode: %s (use enforce, audit, or lockdown)", mode), 1)
	}
}

func handleAuth(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: plasma-shield auth <login|logout>")
		os.Exit(1)
	}

	action := args[0]
	switch action {
	case "login":
		if jsonOut {
			outputJSON(map[string]string{
				"message": "Authentication not yet implemented",
				"hint":    "Set PLASMA_TOKEN environment variable or use --token flag",
			})
		} else {
			fmt.Println("Authentication not yet implemented")
			fmt.Println("Set PLASMA_TOKEN environment variable or use --token flag")
		}
	case "logout":
		if jsonOut {
			outputJSON(map[string]string{
				"message": "Logged out (clear PLASMA_TOKEN to complete)",
			})
		} else {
			fmt.Println("Logged out (clear PLASMA_TOKEN to complete)")
		}
	default:
		exitError(fmt.Sprintf("unknown auth action: %s", action), 1)
	}
}

func handleAccess(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: plasma-shield access <grant|list|revoke> [options]")
		os.Exit(1)
	}

	action := args[0]

	switch action {
	case "grant":
		// Parse grant flags
		grantFlags := flag.NewFlagSet("access grant", flag.ExitOnError)
		target := grantFlags.String("target", "", "Target agent or fleet pattern")
		duration := grantFlags.String("duration", "30m", "Grant duration (e.g., 30m, 1h, 24h)")
		principal := grantFlags.String("principal", "*", "Who can use this grant (default: anyone)")

		grantFlags.Parse(args[1:])

		if *target == "" {
			exitError("--target is required", 1)
		}

		reqBody := map[string]interface{}{
			"target":     *target,
			"duration":   *duration,
			"principal":  *principal,
			"created_by": "cli",
		}

		respBody, statusCode, err := apiRequest("POST", "/grants", reqBody)
		if err != nil {
			exitError(err.Error(), 1)
		}

		if statusCode != http.StatusCreated && statusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(respBody, &errResp)
			exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
		}

		var response struct {
			Grant struct {
				ID        string    `json:"id"`
				Principal string    `json:"principal"`
				Target    string    `json:"target"`
				ExpiresAt time.Time `json:"expires_at"`
				CreatedBy string    `json:"created_by"`
				CreatedAt time.Time `json:"created_at"`
			} `json:"grant"`
			Message string `json:"message"`
		}
		json.Unmarshal(respBody, &response)

		if jsonOut {
			outputJSON(response)
		} else {
			fmt.Printf("✓ Grant created: %s\n", response.Grant.ID)
			fmt.Printf("  Target: %s\n", response.Grant.Target)
			fmt.Printf("  Principal: %s\n", response.Grant.Principal)
			fmt.Printf("  Expires: %s\n", response.Grant.ExpiresAt.Format("2006-01-02 15:04:05 UTC"))
		}

	case "list":
		// Parse list flags
		listFlags := flag.NewFlagSet("access list", flag.ExitOnError)
		activeOnly := listFlags.Bool("active", true, "Show only active grants")

		listFlags.Parse(args[1:])

		query := ""
		if *activeOnly {
			query = "?active=true"
		}

		respBody, statusCode, err := apiRequest("GET", "/grants"+query, nil)
		if err != nil {
			exitError(err.Error(), 1)
		}

		if statusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(respBody, &errResp)
			exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
		}

		var response struct {
			Grants []struct {
				ID        string    `json:"id"`
				Principal string    `json:"principal"`
				Target    string    `json:"target"`
				ExpiresAt time.Time `json:"expires_at"`
				CreatedBy string    `json:"created_by"`
				CreatedAt time.Time `json:"created_at"`
			} `json:"grants"`
			Total int `json:"total"`
		}

		if err := json.Unmarshal(respBody, &response); err != nil {
			exitError(fmt.Sprintf("failed to parse response: %v", err), 1)
		}

		if jsonOut {
			outputJSON(response)
		} else {
			if response.Total == 0 {
				fmt.Println("No active grants")
			} else {
				fmt.Printf("Grants (%d total):\n", response.Total)
				fmt.Println("─────────────────────────────────────────────────────────────")
				for _, grant := range response.Grants {
					remaining := time.Until(grant.ExpiresAt).Round(time.Second)
					status := "✓"
					if remaining <= 0 {
						status = "✗"
						remaining = 0
					}
					fmt.Printf("%s %-24s → %-20s (expires in %s)\n",
						status, grant.ID, grant.Target, remaining)
				}
			}
		}

	case "revoke":
		if len(args) < 2 {
			exitError("grant ID required: plasma-shield access revoke <grant-id>", 1)
		}
		grantID := args[1]

		respBody, statusCode, err := apiRequest("DELETE", "/grants/"+grantID, nil)
		if err != nil {
			exitError(err.Error(), 1)
		}

		if statusCode != http.StatusOK {
			var errResp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(respBody, &errResp)
			exitError(fmt.Sprintf("API error: %s", errResp.Error), 1)
		}

		var response struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		}
		json.Unmarshal(respBody, &response)

		if jsonOut {
			outputJSON(response)
		} else {
			fmt.Printf("✓ %s\n", response.Message)
		}

	default:
		exitError(fmt.Sprintf("unknown access action: %s (use grant, list, or revoke)", action), 1)
	}
}

// Command gateway runs the full Plasma Shield: forward proxy (outbound) + reverse proxy (inbound).
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Extra-Chill/plasma-shield/internal/fleet"
	"github.com/Extra-Chill/plasma-shield/internal/mode"
	"github.com/Extra-Chill/plasma-shield/internal/proxy"
	"github.com/Extra-Chill/plasma-shield/internal/rules"
)

func main() {
	// Flags
	outboundPort := flag.String("outbound", ":8080", "Forward proxy port (outbound agent traffic)")
	inboundPort := flag.String("inbound", ":8443", "Reverse proxy port (inbound to agents)")
	rulesFile := flag.String("rules", "", "Rules file (YAML)")
	agentsFile := flag.String("agents", "/etc/plasma-shield/agents.yaml", "Agents/fleet config file")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("Plasma Shield Gateway starting...")

	// Initialize components
	modeManager := mode.NewManager()
	modeManager.SetGlobalMode(mode.Enforce)

	rulesEngine := rules.NewEngine()
	if *rulesFile != "" {
		if err := rulesEngine.LoadRules(*rulesFile); err != nil {
			log.Printf("Warning: failed to load rules from %s: %v", *rulesFile, err)
		} else {
			log.Printf("Loaded %d rules from %s", rulesEngine.RuleCount(), *rulesFile)
		}
	}

	fleetMgr := fleet.NewManager()

	// Create handlers
	inspector := proxy.NewInspector(rulesEngine, modeManager)
	forwardHandler := proxy.NewHandler(inspector)
	reverseHandler := proxy.NewReverseHandler(fleetMgr)

	// Load fleet config (agents, tokens)
	if err := loadFleetConfig(fleetMgr, reverseHandler, *agentsFile); err != nil {
		log.Printf("Warning: failed to load fleet config from %s: %v", *agentsFile, err)
		// Also try loading tokens from environment as fallback
		loadTokens(reverseHandler)
	}

	// Create servers
	outboundServer := &http.Server{
		Addr:         *outboundPort,
		Handler:      forwardHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	inboundServer := &http.Server{
		Addr:         *inboundPort,
		Handler:      reverseHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// Start servers
	go func() {
		log.Printf("Forward proxy (outbound) listening on %s", *outboundPort)
		if err := outboundServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Forward proxy error: %v", err)
		}
	}()

	go func() {
		log.Printf("Reverse proxy (inbound) listening on %s", *inboundPort)
		if err := inboundServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Reverse proxy error: %v", err)
		}
	}()

	log.Println("Plasma Shield Gateway running")
	log.Println("  Outbound (forward proxy):", *outboundPort)
	log.Println("  Inbound (reverse proxy):", *inboundPort)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	outboundServer.Shutdown(ctx)
	inboundServer.Shutdown(ctx)
	log.Println("Shutdown complete")
}

// loadFleetConfig loads fleet/agent configuration from YAML.
func loadFleetConfig(mgr *fleet.Manager, reverseHandler *proxy.ReverseHandler, path string) error {
	config, err := fleet.LoadConfig(path)
	if err != nil {
		return err
	}

	// Apply fleet/tenant config
	fleet.ApplyConfig(mgr, config)

	// Register tokens for reverse proxy
	for _, tc := range config.Tokens {
		reverseHandler.RegisterToken(tc.Token, tc.TenantID)
		log.Printf("Registered token for tenant: %s (%s)", tc.TenantID, tc.Name)
	}

	// Log loaded config
	for _, tenant := range config.Tenants {
		log.Printf("Loaded tenant: %s (mode=%s, agents=%d)", tenant.ID, tenant.Mode, len(tenant.Agents))
		for _, agent := range tenant.Agents {
			log.Printf("  Agent: %s (%s) -> %s", agent.ID, agent.Name, agent.IP)
		}
	}

	return nil
}

// loadTokens loads auth tokens for the reverse proxy.
func loadTokens(h *proxy.ReverseHandler) {
	// Load from environment
	// Format: SHIELD_TOKEN_<tenant>=<token>
	for _, env := range os.Environ() {
		if len(env) > 13 && env[:13] == "SHIELD_TOKEN_" {
			parts := splitFirst(env[13:], "=")
			if len(parts) == 2 {
				tenantID := parts[0]
				token := parts[1]
				h.RegisterToken(token, tenantID)
				log.Printf("Registered token for tenant: %s", tenantID)
			}
		}
	}
}

func splitFirst(s, sep string) []string {
	idx := -1
	for i := 0; i < len(s)-len(sep)+1; i++ {
		if s[i:i+len(sep)] == sep {
			idx = i
			break
		}
	}
	if idx == -1 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+len(sep):]}
}

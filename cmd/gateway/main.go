// Command gateway runs the full Plasma Shield: forward proxy (outbound) + reverse proxy (inbound).
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Extra-Chill/plasma-shield/internal/bastion"
	"github.com/Extra-Chill/plasma-shield/internal/fleet"
	"github.com/Extra-Chill/plasma-shield/internal/mode"
	"github.com/Extra-Chill/plasma-shield/internal/proxy"
	"github.com/Extra-Chill/plasma-shield/internal/rules"
)

func main() {
	// Flags
	outboundPort := flag.String("outbound", ":8080", "Forward proxy port (outbound agent traffic)")
	inboundPort := flag.String("inbound", ":8443", "Reverse proxy port (inbound to agents)")
	bastionAddr := flag.String("bastion", "", "SSH bastion address (disabled if empty)")
	dataDir := flag.String("data-dir", "/var/lib/plasma-shield", "Directory for persistent state (keys, grants)")
	tlsCert := flag.String("tls-cert", "", "TLS certificate file for inbound HTTPS")
	tlsKey := flag.String("tls-key", "", "TLS private key file for inbound HTTPS")
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
	reverseHandler := proxy.NewReverseHandler(fleetMgr)

	// Load fleet config (agents, tokens) BEFORE creating forward handler
	// so the agent registry is populated
	if err := loadFleetConfig(fleetMgr, reverseHandler, *agentsFile); err != nil {
		log.Printf("Warning: failed to load fleet config from %s: %v", *agentsFile, err)
		// Also try loading tokens from environment as fallback
		loadTokens(reverseHandler)
	}

	// Create forward handler with agent registry for IP validation
	forwardHandler := proxy.NewHandler(inspector, proxy.WithAgentRegistry(fleetMgr))

	// Ensure data directory exists
	if err := os.MkdirAll(*dataDir, 0700); err != nil {
		log.Fatalf("Failed to create data directory %s: %v", *dataDir, err)
	}

	// Initialize SSH bastion (if enabled)
	var bastionServer *bastion.Server
	if *bastionAddr != "" {
		bastionLogStore := bastion.NewLogStore(bastion.DefaultLogLimit)
		bastionLogger := bastion.NewLogger(bastionLogStore)
		bastionGrantStore := bastion.NewGrantStore(filepath.Join(*dataDir, "bastion_grants.json"))

		server, err := bastion.NewServer(bastion.Config{
			Addr:        *bastionAddr,
			HostKeyPath: filepath.Join(*dataDir, "bastion_host_key"),
			CAKeyPath:   filepath.Join(*dataDir, "bastion_ca_key"),
			Logger:      bastionLogger,
			GrantStore:  bastionGrantStore,
		})
		if err != nil {
			log.Fatalf("Failed to initialize bastion: %v", err)
		}
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start bastion: %v", err)
		}
		bastionServer = server
		log.Printf("SSH bastion listening on %s", bastionServer.Addr())
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
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		},
	}

	// Start servers
	go func() {
		log.Printf("Forward proxy (outbound) listening on %s", *outboundPort)
		if err := outboundServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Forward proxy error: %v", err)
		}
	}()

	go func() {
		if *tlsCert != "" && *tlsKey != "" {
			// TLS enabled for inbound
			log.Printf("Reverse proxy (inbound) listening on %s with TLS", *inboundPort)
			if err := inboundServer.ListenAndServeTLS(*tlsCert, *tlsKey); err != http.ErrServerClosed {
				log.Fatalf("Reverse proxy TLS error: %v", err)
			}
		} else {
			// Plain HTTP (not recommended for production)
			log.Printf("WARNING: Reverse proxy (inbound) listening on %s WITHOUT TLS", *inboundPort)
			log.Printf("WARNING: Bearer tokens will be transmitted in plain text!")
			log.Printf("WARNING: Use --tls-cert and --tls-key for production")
			if err := inboundServer.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("Reverse proxy error: %v", err)
			}
		}
	}()

	tlsStatus := "disabled (WARNING: insecure)"
	if *tlsCert != "" && *tlsKey != "" {
		tlsStatus = "enabled"
	}
	log.Println("Plasma Shield Gateway running")
	log.Println("  Outbound (forward proxy):", *outboundPort)
	log.Println("  Inbound (reverse proxy):", *inboundPort, "TLS:", tlsStatus)
	if bastionServer != nil {
		log.Println("  SSH bastion:", bastionServer.Addr())
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	outboundServer.Shutdown(ctx)
	inboundServer.Shutdown(ctx)
	if bastionServer != nil {
		bastionServer.Close()
	}
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

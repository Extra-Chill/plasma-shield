// Plasma Shield Proxy
//
// The core router service that inspects and filters all agent traffic.
// Run this on a dedicated VPS that agents cannot access directly.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Extra-Chill/plasma-shield/internal/mode"
	"github.com/Extra-Chill/plasma-shield/internal/proxy"
	"github.com/Extra-Chill/plasma-shield/internal/rules"
)

var version = "0.1.0"

func main() {
	// Parse command line flags
	proxyAddr := flag.String("proxy-addr", ":8080", "Address for the proxy server")
	apiAddr := flag.String("api-addr", ":8081", "Address for the management API")
	rulesFile := flag.String("rules", "", "Path to rules YAML file")
	flag.Parse()

	fmt.Printf("Plasma Shield Proxy v%s\n", version)

	// Initialize mode manager
	modeManager := mode.NewManager()
	log.Printf("Default mode: %s", modeManager.GlobalMode())

	// Initialize rule engine
	engine := rules.NewEngine()
	if *rulesFile != "" {
		if err := engine.LoadRules(*rulesFile); err != nil {
			log.Fatalf("Failed to load rules: %v", err)
		}
		log.Printf("Loaded rules from %s", *rulesFile)
	}

	// Create inspector and handlers
	inspector := proxy.NewInspector(engine, modeManager)
	proxyHandler := proxy.NewHandler(inspector)
	execCheckHandler := proxy.NewExecCheckHandler(inspector)

	// Create proxy server
	proxyServer := &http.Server{
		Addr:         *proxyAddr,
		Handler:      proxyHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Create API server with management endpoints
	apiMux := http.NewServeMux()
	apiMux.Handle("/exec/check", execCheckHandler)
	apiMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Mode management endpoints
	apiMux.HandleFunc("/mode", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Get current global mode
			resp := map[string]interface{}{
				"global_mode":  string(modeManager.GlobalMode()),
				"agent_modes": modeManager.AllAgentModes(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case http.MethodPut, http.MethodPost:
			// Set global mode
			var req struct {
				Mode string `json:"mode"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			switch mode.Mode(req.Mode) {
			case mode.Enforce, mode.Audit, mode.Lockdown:
				modeManager.SetGlobalMode(mode.Mode(req.Mode))
				log.Printf("Global mode changed to: %s", req.Mode)
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "ok", "mode": req.Mode})
			default:
				http.Error(w, "Invalid mode. Use: enforce, audit, lockdown", http.StatusBadRequest)
			}

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	apiMux.HandleFunc("/agent/", func(w http.ResponseWriter, r *http.Request) {
		// Extract agent ID from path: /agent/{id}/mode
		// Simple implementation - production would use a router
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "not yet implemented"})
	})

	apiServer := &http.Server{
		Addr:         *apiAddr,
		Handler:      apiMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Start servers
	go func() {
		log.Printf("Starting proxy server on %s", *proxyAddr)
		if err := proxyServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Proxy server error: %v", err)
		}
	}()

	go func() {
		log.Printf("Starting API server on %s", *apiAddr)
		if err := apiServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("API server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	proxyServer.Shutdown(ctx)
	apiServer.Shutdown(ctx)

	log.Println("Shutdown complete")
}

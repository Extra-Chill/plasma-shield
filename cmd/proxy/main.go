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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Extra-Chill/plasma-shield/internal/fleet"
	"github.com/Extra-Chill/plasma-shield/internal/mode"
	"github.com/Extra-Chill/plasma-shield/internal/proxy"
	"github.com/Extra-Chill/plasma-shield/internal/rules"
	"github.com/Extra-Chill/plasma-shield/internal/web"
)

var version = "0.1.0"

// LogStore stores recent traffic logs in memory
type LogStore struct {
	mu      sync.RWMutex
	entries []proxy.LogEntry
	maxSize int
}

func NewLogStore(maxSize int) *LogStore {
	return &LogStore{
		entries: make([]proxy.LogEntry, 0),
		maxSize: maxSize,
	}
}

func (s *LogStore) Add(entry proxy.LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry)
	if len(s.entries) > s.maxSize {
		s.entries = s.entries[len(s.entries)-s.maxSize:]
	}
}

func (s *LogStore) Get(limit int) []proxy.LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.entries) {
		limit = len(s.entries)
	}
	// Return most recent first
	result := make([]proxy.LogEntry, limit)
	for i := 0; i < limit; i++ {
		result[i] = s.entries[len(s.entries)-1-i]
	}
	return result
}

func main() {
	// Parse command line flags
	proxyAddr := flag.String("proxy-addr", ":8080", "Address for the proxy server")
	apiAddr := flag.String("api-addr", "127.0.0.1:9000", "Address for the management API and web UI (localhost only)")
	rulesFile := flag.String("rules", "", "Path to rules YAML file")
	flag.Parse()

	fmt.Printf("Plasma Shield Proxy v%s\n", version)

	// Initialize components
	modeManager := mode.NewManager()
	fleetManager := fleet.NewManager()
	logStore := NewLogStore(1000)
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

	// Health check
	apiMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Exec check (for agents)
	apiMux.Handle("/exec/check", execCheckHandler)

	// Mode management
	apiMux.HandleFunc("/mode", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		switch r.Method {
		case http.MethodGet:
			resp := map[string]interface{}{
				"global_mode":  string(modeManager.GlobalMode()),
				"agent_modes":  modeManager.AllAgentModes(),
			}
			json.NewEncoder(w).Encode(resp)

		case http.MethodPut, http.MethodPost:
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
				json.NewEncoder(w).Encode(map[string]string{"status": "ok", "mode": req.Mode})
			default:
				http.Error(w, "Invalid mode. Use: enforce, audit, lockdown", http.StatusBadRequest)
			}

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Per-agent mode management
	apiMux.HandleFunc("/agent/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Parse path: /agent/{id}/mode
		path := strings.TrimPrefix(r.URL.Path, "/agent/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 || parts[1] != "mode" {
			http.Error(w, "Invalid path. Use: /agent/{id}/mode", http.StatusBadRequest)
			return
		}
		agentID := parts[0]

		switch r.Method {
		case http.MethodGet:
			agentMode := modeManager.AgentMode(agentID)
			json.NewEncoder(w).Encode(map[string]string{
				"agent": agentID,
				"mode":  string(agentMode),
			})

		case http.MethodPut:
			var req struct {
				Mode string `json:"mode"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			switch mode.Mode(req.Mode) {
			case mode.Enforce, mode.Audit, mode.Lockdown:
				modeManager.SetAgentMode(agentID, mode.Mode(req.Mode))
				log.Printf("Agent %s mode changed to: %s", agentID, req.Mode)
				json.NewEncoder(w).Encode(map[string]string{"status": "ok", "agent": agentID, "mode": req.Mode})
			default:
				http.Error(w, "Invalid mode", http.StatusBadRequest)
			}

		case http.MethodDelete:
			modeManager.ClearAgentMode(agentID)
			log.Printf("Agent %s mode cleared", agentID)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "agent": agentID, "message": "mode cleared"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Traffic logs
	apiMux.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				limit = n
			}
		}

		logs := logStore.Get(limit)
		json.NewEncoder(w).Encode(logs)
	})

	// Rules management
	apiMux.HandleFunc("/rules", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get rules from the engine
		// For now, return the rules file path info
		resp := map[string]interface{}{
			"rules_path": engine.RulesPath(),
			"rule_count": engine.RuleCount(),
			"rules":      []interface{}{}, // TODO: expose rules from engine
		}
		json.NewEncoder(w).Encode(resp)
	})

	// Fleet management
	apiMux.HandleFunc("/fleet/mode", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		tenantID := r.URL.Query().Get("tenant")
		if tenantID == "" {
			tenantID = "default"
		}

		switch r.Method {
		case http.MethodGet:
			mode := fleetManager.GetMode(tenantID)
			json.NewEncoder(w).Encode(map[string]string{
				"tenant": tenantID,
				"mode":   string(mode),
			})

		case http.MethodPut:
			var req struct {
				Mode string `json:"mode"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			switch fleet.Mode(req.Mode) {
			case fleet.Isolated, fleet.Fleet:
				fleetManager.SetMode(tenantID, fleet.Mode(req.Mode))
				log.Printf("Tenant %s fleet mode changed to: %s", tenantID, req.Mode)
				json.NewEncoder(w).Encode(map[string]string{
					"status": "ok",
					"tenant": tenantID,
					"mode":   req.Mode,
				})
			default:
				http.Error(w, "Invalid mode. Use: isolated, fleet", http.StatusBadRequest)
			}

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	apiMux.HandleFunc("/fleet/agents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		tenantID := r.URL.Query().Get("tenant")
		if tenantID == "" {
			tenantID = "default"
		}

		switch r.Method {
		case http.MethodGet:
			// Get agents - respects fleet mode (returns empty in isolated mode)
			agents := fleetManager.GetAgents(tenantID)
			mode := fleetManager.GetMode(tenantID)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"tenant": tenantID,
				"mode":   string(mode),
				"agents": agents,
			})

		case http.MethodPost:
			var agent fleet.Agent
			if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if agent.ID == "" {
				http.Error(w, "Agent ID required", http.StatusBadRequest)
				return
			}
			fleetManager.AddAgent(tenantID, agent)
			log.Printf("Agent %s added to tenant %s", agent.ID, tenantID)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "ok",
				"tenant": tenantID,
				"agent":  agent.ID,
			})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	apiMux.HandleFunc("/fleet/can-communicate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")

		if from == "" || to == "" {
			http.Error(w, "from and to parameters required", http.StatusBadRequest)
			return
		}

		canComm := fleetManager.CanCommunicate(from, to)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"from":            from,
			"to":              to,
			"can_communicate": canComm,
		})
	})

	// Serve web UI at root
	apiMux.Handle("/", web.Handler())

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
		log.Printf("Starting API + Web UI on %s", *apiAddr)
		log.Printf("  Web UI: http://localhost%s/", *apiAddr)
		log.Printf("  API:    http://localhost%s/mode", *apiAddr)
		if err := apiServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("API server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	proxyServer.Shutdown(ctx)
	apiServer.Shutdown(ctx)

	log.Println("Shutdown complete")
}

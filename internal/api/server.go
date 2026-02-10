package api

import (
	"context"
	"log"
	"net/http"
	"time"
)

// ServerConfig holds server configuration.
type ServerConfig struct {
	Addr            string
	ManagementToken string
	AgentToken      string
	Version         string
}

// Server is the Plasma Shield management API server.
type Server struct {
	httpServer *http.Server
	handlers   *Handlers
	authConfig *AuthConfig
}

// NewServer creates a new API server.
func NewServer(cfg ServerConfig) *Server {
	store := NewStore()
	handlers := NewHandlers(store, cfg.Version)

	authConfig := &AuthConfig{
		ManagementToken: cfg.ManagementToken,
		AgentToken:      cfg.AgentToken,
	}

	mux := http.NewServeMux()

	// Management endpoints (require management token)
	mux.Handle("/status", applyMiddleware(
		http.HandlerFunc(handlers.StatusHandler),
		ManagementAuth(authConfig),
	))

	mux.Handle("/agents", applyMiddleware(
		http.HandlerFunc(handlers.ListAgentsHandler),
		ManagementAuth(authConfig),
	))

	// Agent action endpoints
	mux.Handle("/agents/", applyMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			switch {
			case hasSuffix(path, "/pause"):
				handlers.PauseAgentHandler(w, r)
			case hasSuffix(path, "/kill"):
				handlers.KillAgentHandler(w, r)
			case hasSuffix(path, "/resume"):
				handlers.ResumeAgentHandler(w, r)
			default:
				writeError(w, http.StatusNotFound, "not found")
			}
		}),
		ManagementAuth(authConfig),
	))

	mux.Handle("/rules", applyMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handlers.ListRulesHandler(w, r)
			case http.MethodPost:
				handlers.CreateRuleHandler(w, r)
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		}),
		ManagementAuth(authConfig),
	))

	mux.Handle("/rules/", applyMiddleware(
		http.HandlerFunc(handlers.DeleteRuleHandler),
		ManagementAuth(authConfig),
	))

	mux.Handle("/logs", applyMiddleware(
		http.HandlerFunc(handlers.ListLogsHandler),
		ManagementAuth(authConfig),
	))

	mux.Handle("/bastion/sessions", applyMiddleware(
		http.HandlerFunc(handlers.ListBastionSessionsHandler),
		ManagementAuth(authConfig),
	))

	mux.Handle("/grants", applyMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handlers.ListGrantsHandler(w, r)
			case http.MethodPost:
				handlers.CreateGrantHandler(w, r)
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		}),
		ManagementAuth(authConfig),
	))

	mux.Handle("/grants/", applyMiddleware(
		http.HandlerFunc(handlers.DeleteGrantHandler),
		ManagementAuth(authConfig),
	))

	// Agent endpoint (requires agent token)
	mux.Handle("/exec/check", applyMiddleware(
		http.HandlerFunc(handlers.ExecCheckHandler),
		AgentAuth(authConfig),
	))

	// Health check (no auth)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	httpServer := &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		httpServer: httpServer,
		handlers:   handlers,
		authConfig: authConfig,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	log.Printf("Starting Plasma Shield API on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// RegisterAgent registers an agent (for testing/setup).
func (s *Server) RegisterAgent(id, name, ip string) {
	s.handlers.RegisterAgent(id, name, ip)
}

// Handlers returns the handlers (for testing).
func (s *Server) Handlers() *Handlers {
	return s.handlers
}

// applyMiddleware applies middleware to a handler.
func applyMiddleware(h http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}

// hasSuffix checks if a path ends with a suffix.
func hasSuffix(path, suffix string) bool {
	return len(path) > len(suffix) && path[len(path)-len(suffix):] == suffix
}

// Plasma Shield Management API
//
// REST API server for human-only shield management.
// Run this on a dedicated VPS with the shield router.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Extra-Chill/plasma-shield/internal/api"
)

var version = "0.1.0"

func main() {
	addr := flag.String("addr", ":8443", "API listen address")
	mgmtToken := flag.String("mgmt-token", "", "Management bearer token (required)")
	agentToken := flag.String("agent-token", "", "Agent bearer token (required)")
	flag.Parse()

	// Allow env vars as fallback
	if *mgmtToken == "" {
		*mgmtToken = os.Getenv("PLASMA_MGMT_TOKEN")
	}
	if *agentToken == "" {
		*agentToken = os.Getenv("PLASMA_AGENT_TOKEN")
	}

	if *mgmtToken == "" || *agentToken == "" {
		log.Fatal("mgmt-token and agent-token are required (flags or PLASMA_MGMT_TOKEN/PLASMA_AGENT_TOKEN env)")
	}

	cfg := api.ServerConfig{
		Addr:            *addr,
		ManagementToken: *mgmtToken,
		AgentToken:      *agentToken,
		Version:         version,
	}

	server := api.NewServer(cfg)

	// Register a demo agent for testing
	server.RegisterAgent("agent-1", "sarai", "178.156.229.129")

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	log.Printf("Plasma Shield API v%s running on %s", version, *addr)
	log.Println("Endpoints:")
	log.Println("  GET  /status         - Shield status")
	log.Println("  GET  /agents         - List agents")
	log.Println("  POST /agents/{id}/pause  - Pause agent")
	log.Println("  POST /agents/{id}/kill   - Kill agent")
	log.Println("  POST /agents/{id}/resume - Resume agent")
	log.Println("  GET  /rules          - List rules")
	log.Println("  POST /rules          - Create rule")
	log.Println("  DELETE /rules/{id}   - Delete rule")
	log.Println("  GET  /logs           - View logs")
	log.Println("  POST /exec/check     - Check command (agent auth)")
	log.Println("  GET  /health         - Health check (no auth)")

	<-stop
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}

	log.Println("Goodbye!")
}

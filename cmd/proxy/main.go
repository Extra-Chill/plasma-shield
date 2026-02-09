// Plasma Shield Proxy
//
// The core router service that inspects and filters all agent traffic.
// Run this on a dedicated VPS that agents cannot access directly.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Initialize rule engine
	engine := rules.NewEngine()
	if *rulesFile != "" {
		if err := engine.LoadRules(*rulesFile); err != nil {
			log.Fatalf("Failed to load rules: %v", err)
		}
		log.Printf("Loaded rules from %s", *rulesFile)
	}

	// Create inspector and handlers
	inspector := proxy.NewInspector(engine)
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

	// Create API server with /exec/check endpoint
	apiMux := http.NewServeMux()
	apiMux.Handle("/exec/check", execCheckHandler)
	apiMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
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

// Plasma Shield CLI
//
// Human-only management interface for the shield router.
// Install on your personal machine, not on agent VPSes.
package main

import (
	"fmt"
	"os"
)

var version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("plasma-shield v%s\n", version)
	case "status":
		fmt.Println("Shield status: not connected")
		fmt.Println("Run 'plasma-shield auth login' to connect to your shield router")
	case "mode":
		if len(os.Args) < 3 {
			fmt.Println("Usage: plasma-shield mode <enforce|audit|lockdown>")
			fmt.Println("\nModes:")
			fmt.Println("  enforce   Normal operation - block matching requests (default)")
			fmt.Println("  audit     Log all requests but never block (testing)")
			fmt.Println("  lockdown  Block ALL outbound requests (emergency)")
			os.Exit(1)
		}
		handleMode(os.Args[2:])
	case "agent":
		if len(os.Args) < 3 {
			fmt.Println("Usage: plasma-shield agent <list|pause|kill|mode> [agent-id] [options]")
			os.Exit(1)
		}
		handleAgent(os.Args[2:])
	case "rules":
		if len(os.Args) < 3 {
			fmt.Println("Usage: plasma-shield rules <list|add|remove> [options]")
			os.Exit(1)
		}
		handleRules(os.Args[2:])
	case "logs":
		handleLogs(os.Args[2:])
	case "auth":
		if len(os.Args) < 3 {
			fmt.Println("Usage: plasma-shield auth <login|logout>")
			os.Exit(1)
		}
		handleAuth(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Plasma Shield CLI - Network security for AI agent fleets

Usage: plasma-shield <command> [options]

Commands:
  status          Show shield connection status
  mode            Set global operating mode (enforce/audit/lockdown)
  agent           Manage agents (list, pause, kill, mode)
  rules           Manage blocking rules
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
  plasma-shield agent mode sarai audit        # Audit mode for one agent
  plasma-shield agent kill sarai              # Emergency stop
  plasma-shield logs --tail --agent sarai

Documentation: https://github.com/Extra-Chill/plasma-shield`)
}

func handleAgent(args []string) {
	fmt.Println("Agent management not yet implemented")
}

func handleRules(args []string) {
	fmt.Println("Rules management not yet implemented")
}

func handleLogs(args []string) {
	fmt.Println("Log viewing not yet implemented")
}

func handleAuth(args []string) {
	fmt.Println("Authentication not yet implemented")
}

func handleMode(args []string) {
	mode := args[0]
	switch mode {
	case "enforce", "audit", "lockdown":
		fmt.Printf("Setting global mode to: %s\n", mode)
		fmt.Println("(API call not yet implemented)")
	default:
		fmt.Printf("Unknown mode: %s\n", mode)
		fmt.Println("Valid modes: enforce, audit, lockdown")
		os.Exit(1)
	}
}

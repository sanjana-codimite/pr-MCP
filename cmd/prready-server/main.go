package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/user/mcp-pr-readiness/internal/config"
	"github.com/user/mcp-pr-readiness/internal/mcpserver"
)

func main() {
	// CRITICAL: With stdio transport, stdout carries the MCP protocol.
	// All logging MUST go to stderr. A stray stdout write breaks the connection.
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	configPath := flag.String("config", "./.prready.json", "Path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	log.Printf("Config loaded: repo=%s base_branch=%s", cfg.RepoPath, cfg.BaseBranch)

	srv := mcpserver.New(cfg)
	if err := srv.Run(context.Background()); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

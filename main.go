package main

import (
	"log"

	"github.com/mark3labs/codebench-mcp/jsserver"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create and start the server
	jss, err := jsserver.NewJSServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Serve requests
	if err := server.ServeStdio(jss); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

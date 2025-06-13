# JavaScript Executor MCP Server

This MCP server provides JavaScript execution capabilities with ski runtime.

## Features

The `executeJS` tool provides:

- **Console API**: `console.log()`, `console.error()`, `console.warn()` (built-in)
- **HTTP Server**: `serve()` for server creation (via `require('ski/http/server')`)
- **Fetch API**: Modern `fetch()` with Request, Response, Headers, FormData (global)
- **Timers**: `setTimeout()`, `setInterval()`, `clearTimeout()`, `clearInterval()` (global)
- **Buffer**: Buffer, Blob, File APIs for binary data handling (global)
- **Crypto**: Cryptographic functions - hashing, encryption, HMAC (via `require('crypto')`)
- **Cache**: In-memory caching with TTL support (via `require('cache')`)
- **Additional modules**: encoding (global), url (global)

## Getting Started

### Installation

#### Using Go Install

```bash
go install github.com/mark3labs/codebench-mcp@latest
```

### Usage

#### As a standalone server

```bash
codebench-mcp
```

#### With module configuration

```bash
# Enable only specific modules
codebench-mcp --enabled-modules http,fetch

# Disable specific modules (enable all others)
codebench-mcp --disabled-modules timers

# Show help
codebench-mcp --help
```

**Available modules:**
- `http` - HTTP server creation and client requests (import serve from 'ski/http/server')
- `fetch` - Modern fetch API with Request, Response, Headers, FormData (available globally)
- `timers` - setTimeout, setInterval, clearTimeout, clearInterval (available globally)
- `buffer` - Buffer, Blob, File APIs for binary data handling (available globally)
- `cache` - In-memory caching with TTL support (require('cache'))
- `crypto` - Cryptographic functions (hashing, encryption, HMAC) (require('crypto'))
- `encoding` - TextEncoder, TextDecoder for text encoding/decoding (available globally)
- `url` - URL and URLSearchParams APIs (available globally)

**Default modules:** `http`, `fetch`, `timers`, `buffer`, `kv`

**Note:** The `executeJS` tool description dynamically updates to show only the enabled modules and includes detailed information about what each module provides.

#### As a library in your Go project

```go
package main

import (
	"log"

	"github.com/mark3labs/codebench-mcp/jsserver"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create a new JavaScript executor server
	jss, err := jsserver.NewJSServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Serve requests
	if err := server.ServeStdio(jss); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
```

#### Using InProcessTransport

```go
package main

import (
	"context"
	"log"

	"github.com/mark3labs/codebench-mcp/jsserver"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func main() {
	// Create the JS server with custom module configuration
	config := jsserver.ModuleConfig{
		EnabledModules: []string{"fetch", "crypto", "buffer"},
	}
	jsServer, err := jsserver.NewJSServerWithConfig(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Create an in-process client
	mcpClient, err := client.NewInProcessClient(jsServer)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()

	// Start the client
	if err := mcpClient.Start(context.Background()); err != nil {
		log.Fatalf("Failed to start client: %v", err)
	}

	// Initialize the client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "my-app",
		Version: "1.0.0",
	}
	_, err = mcpClient.Initialize(context.Background(), initRequest)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// Execute JavaScript code
	callRequest := mcp.CallToolRequest{}
	callRequest.Params.Name = "executeJS"
	callRequest.Params.Arguments = map[string]any{
		"code": `
			console.log("Hello from JavaScript!");
			const result = Math.sqrt(16);
			console.log("Square root of 16 is:", result);
			result;
		`,
	}

	result, err := mcpClient.CallTool(context.Background(), callRequest)
	if err != nil {
		log.Fatalf("Failed to call tool: %v", err)
	}

	if result.IsError {
		log.Printf("JavaScript execution error: %v", result.Content)
	} else {
		log.Printf("JavaScript execution result: %v", result.Content)
	}
}
```

### Usage with Model Context Protocol

To integrate this server with apps that support MCP:

```json
{
  "mcpServers": {
    "javascript": {
      "command": "codebench-mcp"
    }
  }
}
```

### Docker

#### Running with Docker

You can run the JavaScript Executor MCP server using Docker:

```bash
docker run -i --rm ghcr.io/mark3labs/codebench-mcp:latest
```

#### Docker Configuration with MCP

To integrate the Docker image with apps that support MCP:

```json
{
  "mcpServers": {
    "javascript": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "ghcr.io/mark3labs/codebench-mcp:latest"
      ]
    }
  }
}
```

## Tool Reference

### executeJS

Execute JavaScript code with ski runtime environment.

**Parameters:**
- `code` (required): JavaScript code to execute

**Example:**
```javascript
console.log("Hello, World!");

// Basic JavaScript execution
const result = 2 + 3;
console.log('Result:', result);

// Fetch API (available globally when enabled)
const response = await fetch('https://api.example.com/data');
const data = await response.json();

// HTTP server (require import)
const serve = require('ski/http/server');
serve(8000, async (req) => {
  return new Response('Hello World');
});

// Cache operations (require import)
const cache = require('cache');
cache.set('key', 'value');
console.log(cache.get('key'));

// Crypto operations (require import)
const crypto = require('crypto');
const hash = crypto.md5('hello').hex();
console.log('MD5 hash:', hash);

// Timers (available globally)
setTimeout(() => console.log('Hello after 1 second'), 1000);

// Buffer operations (available globally)
const buffer = Buffer.from('hello', 'utf8');
console.log(buffer.toString('base64'));

// URL operations (available globally)
const url = new URL('https://example.com/path?param=value');
console.log('Host:', url.host);
console.log('Pathname:', url.pathname);
```

## Limitations

- **No fs or process modules** - File system and process APIs are not available in ski runtime
- **Module access varies** - Some modules are global (fetch, http), others may need require()
- **Each execution creates a fresh VM** - For isolation, each execution starts with a clean state
- **Module filtering** - Configuration exists but actual runtime filtering not fully implemented

## Building

```bash
go build -o codebench-mcp .
```

## License

See the [LICENSE](LICENSE) file for details.
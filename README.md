# JavaScript Executor MCP Server

This MCP server provides JavaScript execution capabilities with a Node.js-like environment.

## Features

The `executeJS` tool provides:

- **Console API**: `console.log()`, `console.error()`, `console.warn()`
- **File System**: `fs.readFileSync()`, `fs.writeFileSync()`, `fs.existsSync()`
- **HTTP Server**: `http.createServer()` with request/response handling
- **Fetch API**: `fetch()` with Promise support for HTTP requests
- **Timers**: `setTimeout()`, `clearTimeout()`, `setInterval()`, `clearInterval()`
- **Process**: `process.argv`, `process.cwd()`, `process.exit()`, `process.env`
- **Module System**: `require()` for loading JavaScript modules

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
codebench-mcp --enabled-modules console,fs,timers

# Disable specific modules (enable all others)
codebench-mcp --disabled-modules http,fetch

# Show help
codebench-mcp --help
```

**Available modules:**
- `console` - Console logging (console.log, console.error, console.warn)
- `fs` - File system operations (fs.readFileSync, fs.writeFileSync, fs.existsSync)
- `http` - HTTP server creation (http.createServer)
- `fetch` - HTTP client requests (fetch API with promises)
- `timers` - Timer functions (setTimeout, setInterval, clearTimeout, clearInterval)
- `process` - Process information (process.argv, process.cwd, process.env, process.exit)
- `require` - Module loading system

**Note:** The `executeJS` tool description dynamically updates to show only the enabled modules and includes detailed information about what each module provides. This helps users understand exactly what JavaScript APIs are available in the simplified VM environment.

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
	// Create the JS server
	jsServer, err := jsserver.NewJSServer()
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

Execute JavaScript code with full Node.js-like environment.

**Parameters:**
- `code` (required): JavaScript code to execute

**Example:**
```javascript
console.log("Hello, World!");

// File operations
fs.writeFileSync("test.txt", "Hello from JS!");
const content = fs.readFileSync("test.txt");
console.log("File content:", content);

// Fetch API with promises
fetch("https://api.github.com/users/octocat")
  .then(response => response.json())
  .then(data => {
    console.log("User:", data.name);
    console.log("Public repos:", data.public_repos);
  })
  .catch(error => console.error("Fetch error:", error));

// HTTP server with configurable ports and callbacks
const server = http.createServer((req, res) => {
  console.log(`${req.method} ${req.url}`);
  
  res.setHeader("Content-Type", "application/json");
  res.writeHead(200);
  res.end(JSON.stringify({
    message: "Hello from HTTP server!",
    method: req.method,
    url: req.url
  }));
});

// Multiple ways to start the server:
server.listen(3000);                           // Port only
server.listen(3000, () => console.log("Started!")); // Port + callback
server.listen(3000, "localhost");             // Port + host
server.listen(3000, "localhost", () => {      // Port + host + callback
  console.log("Server running on localhost:3000");
});

// Server management
setTimeout(() => {
  server.close(); // Gracefully shutdown server
}, 10000);

// Timers
setTimeout(() => {
  console.log("Timer executed!");
}, 1000);

// Process info
console.log("Current directory:", process.cwd());
console.log("Arguments:", process.argv);
```

## Building

```bash
go build -o codebench-mcp .
```

## License

See the [LICENSE](LICENSE) file for details.
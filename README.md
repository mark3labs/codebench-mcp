# JavaScript Executor MCP Server

This MCP server provides JavaScript execution capabilities with a Node.js-like environment.

## Features

The `executeJS` tool provides:

- **Console API**: `console.log()`, `console.error()`, `console.warn()`
- **File System**: `fs.readFileSync()`, `fs.writeFileSync()`, `fs.existsSync()`
- **HTTP Server**: `http.createServer()` with request/response handling
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

// HTTP server
const server = http.createServer((req, res) => {
  res.end("Hello from HTTP server!");
});
server.listen(8080);

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
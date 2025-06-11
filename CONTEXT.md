# CodeBench MCP - JavaScript Executor

An MCP server that provides JavaScript execution capabilities with ski runtime.

## Tool

### executeJS
- Execute JavaScript code with ski runtime environment
- Includes: console (built-in), http, fetch, timers, buffer, crypto, and other ski modules
- Parameters: `code` (required): JavaScript code to execute
- Modules are configurable via CLI flags

## CLI Usage
- `codebench-mcp` - Run with default modules enabled (http, fetch, timers, buffer, crypto)
- `codebench-mcp --enabled-modules http,fetch` - Enable only specific modules
- `codebench-mcp --disabled-modules timers` - Disable specific modules
- Available modules: http (import serve from 'ski/http/server'), fetch (global), timers (global), buffer (global), cache (import cache from 'ski/cache'), crypto (import crypto from 'ski/crypto'), dom, encoding (global), ext, html, signal (global), stream (global), url (global)

## Build/Test Commands
- `go build ./...` - Build all packages
- `go test ./...` - Run all tests
- `go test -v ./...` - Run tests with verbose output
- `go test -run TestName` - Run specific test
- `go mod tidy` - Clean up dependencies
- `go fmt ./...` - Format code
- `go vet ./...` - Static analysis
- `golangci-lint run` - Comprehensive linting (if configured)

## Code Style Guidelines
- Use `gofmt` for consistent formatting
- Follow standard Go naming conventions: PascalCase for exported, camelCase for unexported
- Package names should be lowercase, single words when possible
- Use meaningful variable names, avoid abbreviations
- Error handling: always check errors, wrap with context using `fmt.Errorf("context: %w", err)`
- Imports: group standard library, third-party, then local packages with blank lines
- Use interfaces for testability and loose coupling
- Prefer composition over inheritance
- Keep functions small and focused on single responsibility

## Project Info
- Go version: 1.23.10
- Module: github.com/mark3labs/codebench-mcp

## Known Limitations
- No fs or process modules - not available in ski runtime
- Module access varies: some modules are global (fetch, timers, buffer, encoding, signal, stream, url), others require imports (http: 'ski/http/server', cache: 'ski/cache', crypto: 'ski/crypto')
- Each execution creates a fresh VM instance for isolation
- Module filtering configuration exists but actual runtime filtering not fully implemented
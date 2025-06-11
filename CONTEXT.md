# CodeBench MCP - JavaScript Executor

An MCP server that provides JavaScript execution capabilities with Node.js-like APIs.

## Tool

### executeJS
- Execute JavaScript code with full Node.js-like environment
- Includes: console, fs, http, fetch (with promises), timers (setTimeout/setInterval), process, and require
- Parameters: `code` (required): JavaScript code to execute
- Modules are configurable via CLI flags

## CLI Usage
- `codebench-mcp` - Run with all modules enabled
- `codebench-mcp --enabled-modules console,fs,timers` - Enable only specific modules
- `codebench-mcp --disabled-modules http,fetch` - Disable specific modules
- Available modules: console, fs, http, fetch, timers, process, require

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
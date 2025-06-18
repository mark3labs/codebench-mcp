package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/grafana/sobek"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	// Import our new VM system
	"github.com/mark3labs/codebench-mcp/internal/logger"
	"github.com/mark3labs/codebench-mcp/server/modules/buffer"
	"github.com/mark3labs/codebench-mcp/server/modules/cache"
	"github.com/mark3labs/codebench-mcp/server/modules/console"
	"github.com/mark3labs/codebench-mcp/server/modules/crypto"
	"github.com/mark3labs/codebench-mcp/server/modules/encoding"
	"github.com/mark3labs/codebench-mcp/server/modules/fetch"
	"github.com/mark3labs/codebench-mcp/server/modules/http"
	"github.com/mark3labs/codebench-mcp/server/modules/kv"
	"github.com/mark3labs/codebench-mcp/server/modules/timers"
	"github.com/mark3labs/codebench-mcp/server/modules/url"
	"github.com/mark3labs/codebench-mcp/server/vm"
)

var Version = "dev"

type ModuleConfig struct {
	EnabledModules   []string
	DisabledModules  []string
	ExecutionTimeout time.Duration
}

type JSHandler struct {
	vmManager    *vm.VMManager
	config       ModuleConfig
	runningVMs   []*vm.VM
	vmMutex      sync.Mutex
}

func NewJSHandler() *JSHandler {
	return NewJSHandlerWithConfig(ModuleConfig{
		EnabledModules:   []string{"http", "fetch", "timers", "buffer", "kv", "crypto", "encoding", "url", "cache"},
		ExecutionTimeout: 5 * time.Minute,
	})
}

func NewJSHandlerWithConfig(config ModuleConfig) *JSHandler {
	// Create VM manager with enabled modules
	enabledModules := config.EnabledModules
	if len(enabledModules) == 0 && len(config.DisabledModules) == 0 {
		// Enable all modules by default if none specified
		enabledModules = []string{"http", "fetch", "timers", "buffer", "kv", "crypto", "encoding", "url", "cache"}
	}

	vmManager := vm.NewVMManager(enabledModules)

	// Register all available modules (except console which is handled per-execution)
	vmManager.RegisterModule(kv.NewKVModule())
	vmManager.RegisterModule(timers.NewTimersModule())
	vmManager.RegisterModule(fetch.NewFetchModule())
	vmManager.RegisterModule(buffer.NewBufferModule())
	vmManager.RegisterModule(http.NewHTTPModule())
	vmManager.RegisterModule(crypto.NewCryptoModule())
	vmManager.RegisterModule(encoding.NewEncodingModule())
	vmManager.RegisterModule(url.NewURLModule())
	vmManager.RegisterModule(cache.NewCacheModule())

	return &JSHandler{
		vmManager: vmManager,
		config:    config,
	}
}

func (h *JSHandler) handleExecuteJS(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	code, err := request.RequireString("code")
	if err != nil {
		return nil, err
	}

	logger.Debug("Executing JavaScript code", "length", len(code))

	// Check if this looks like HTTP server code
	isServerCode := strings.Contains(code, "serve(") && 
		(strings.Contains(code, "require('http/server')") || 
		 strings.Contains(code, "require(\"http/server\")"))

	if isServerCode {
		logger.Debug("Detected server code, running in background")
		// For server code, run in a goroutine and return immediately
		return h.handleServerCode(ctx, code)
	} else {
		logger.Debug("Running regular JavaScript code")
		// For regular code, run synchronously
		return h.handleRegularCode(ctx, code)
	}
}

func (h *JSHandler) handleServerCode(ctx context.Context, code string) (*mcp.CallToolResult, error) {
	// Capture console output
	var output strings.Builder

	// Channel to capture execution results
	resultChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	// Run the server code in a goroutine that stays alive
	go func() {
		// Create VM with custom logger for console output
		// Use background context so VM doesn't get cancelled when request finishes
		vmCtx := context.Background()
		vm, err := h.vmManager.CreateVM(vmCtx)
		if err != nil {
			logger.Debug("Failed to create VM", "error", err)
			errorChan <- err
			return
		}

		// Track this VM for cleanup
		h.vmMutex.Lock()
		h.runningVMs = append(h.runningVMs, vm)
		h.vmMutex.Unlock()

		// Setup console module to capture output
		consoleModule := console.NewConsoleModule(&output)
		consoleModule.Setup(vm.Runtime())

		// Execute the JavaScript code
		_, err = vm.RunString(code)
		if err != nil {
			logger.Error("Server execution error", "error", err)
			errorChan <- err
			// Remove from tracking and close VM on error
			h.vmMutex.Lock()
			for i, trackedVM := range h.runningVMs {
				if trackedVM == vm {
					h.runningVMs = append(h.runningVMs[:i], h.runningVMs[i+1:]...)
					break
				}
			}
			h.vmMutex.Unlock()
			vm.Close()
			return
		}

		// Send initial output back
		resultChan <- output.String()

		// Keep the goroutine and VM alive indefinitely for HTTP servers
		// The VM will be cleaned up when the MCP server shuts down
		select {}
	}()

	// Wait for initial execution to complete or timeout
	select {
	case <-time.After(2 * time.Second):
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Server code execution timeout. If this is an HTTP server, it may still be starting in the background.",
				},
			},
			IsError: true,
		}, nil
	case err := <-errorChan:
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Server execution error: %v", err),
				},
			},
			IsError: true,
		}, nil
	case result := <-resultChan:
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Server code executed in background:\n%s", result),
				},
			},
		}, nil
	}
}

func (h *JSHandler) handleRegularCode(ctx context.Context, code string) (*mcp.CallToolResult, error) {
	// Capture console output
	var output strings.Builder

	// Create VM instance for this execution
	vm, err := h.vmManager.CreateVM(ctx)
	if err != nil {
		logger.Debug("Failed to create VM", "error", err)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to create VM: %v", err),
				},
			},
			IsError: true,
		}, nil
	}
	defer vm.Close()

	// Setup console module to capture output
	consoleModule := console.NewConsoleModule(&output)
	consoleModule.Setup(vm.Runtime())

	// Execute the JavaScript code with configurable timeout
	timeout := h.config.ExecutionTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute // Default fallback
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute in a goroutine to respect timeout
	resultChan := make(chan sobek.Value, 1)
	errorChan := make(chan error, 1)

	go func() {
		result, err := vm.RunString(code)
		if err != nil {
			errorChan <- err
		} else {
			resultChan <- result
		}
	}()

	select {
	case <-execCtx.Done():
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("JavaScript execution timeout\n\nOutput:\n%s", output.String()),
				},
			},
			IsError: true,
		}, nil
	case err := <-errorChan:
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("JavaScript execution error: %v\n\nOutput:\n%s", err, output.String()),
				},
			},
			IsError: true,
		}, nil
	case result := <-resultChan:
		// Get the result value
		var resultStr string
		if result != nil && !sobek.IsUndefined(result) && !sobek.IsNull(result) {
			exported := result.Export()
			if exported != nil {
				resultStr = fmt.Sprintf("Result: %v\n", exported)
			}
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("%s%s", output.String(), resultStr),
				},
			},
		}, nil
	}
}

func (h *JSHandler) getAvailableModules() []string {
	return h.vmManager.GetEnabledModules()
}

// Cleanup shuts down all running VMs
func (h *JSHandler) Cleanup() {
	h.vmMutex.Lock()
	defer h.vmMutex.Unlock()
	
	logger.Debug("Cleaning up running VMs", "count", len(h.runningVMs))
	for _, vm := range h.runningVMs {
		vm.Close()
	}
	h.runningVMs = nil
}

func NewJSServer() (*server.MCPServer, error) {
	return NewJSServerWithConfig(ModuleConfig{
		EnabledModules: []string{"http", "fetch", "timers", "buffer", "kv", "crypto"},
	})
}

func NewJSServerWithConfig(config ModuleConfig) (*server.MCPServer, error) {
	h := NewJSHandlerWithConfig(config)

	s := server.NewMCPServer(
		"codebench-mcp",
		Version,
	)

	// Build detailed description with module information
	description := buildToolDescription(h.getAvailableModules())

	// Register the executeJS tool
	s.AddTool(mcp.NewTool(
		"executeJS",
		mcp.WithDescription(description),
		mcp.WithString("code",
			mcp.Description("Complete JavaScript source code to execute in a modern runtime environment. This parameter accepts a full JavaScript program including variable declarations, function definitions, control flow statements, and module imports via require(). The code will be executed in a sandboxed environment with access to enabled modules. Supports modern JavaScript syntax (ES2020+) including arrow functions, destructuring, template literals, and promises. Use require() for module imports (e.g., 'const serve = require(\"http/server\")') rather than ES6 import statements. Note: Top-level async/await is not supported - wrap async code in an async function and call it (e.g., '(async () => { await fetch(...); })()' or define and call an async function). The execution context includes a console object for output, and any returned values will be displayed along with console output. For HTTP servers, they will run in the background without blocking execution completion."),
			mcp.Required(),
		),
	), h.handleExecuteJS)

	return s, nil
}

func buildToolDescription(enabledModules []string) string {
	var description strings.Builder

	description.WriteString("Execute JavaScript code with Node.js-like APIs powered by a modern runtime. ")
	description.WriteString("Supports modern JavaScript (ES2020+), CommonJS modules via require(), promises, and comprehensive JavaScript APIs. ")
	description.WriteString("ES6 import statements are not supported in direct execution - use require() instead.\n\n")

	if len(enabledModules) == 0 {
		description.WriteString("No modules are currently enabled. Only basic JavaScript execution is available.")
		return description.String()
	}

	description.WriteString("Available modules:\n")

	// Define module descriptions
	moduleDescriptions := map[string]string{
		"http":     "HTTP server creation and management (const serve = require('http/server'))",
		"fetch":    "Modern fetch API with Request, Response, Headers, FormData (available globally)",
		"timers":   "setTimeout, setInterval, clearTimeout, clearInterval (available globally)",
		"buffer":   "Buffer, Blob, File APIs for binary data handling (available globally)",
		"crypto":   "Cryptographic functions (hashing, encryption, HMAC) (const crypto = require('crypto'))",
		"cache":    "In-memory caching with TTL support (const cache = require('cache'))",
		"kv":       "Key-value store per VM instance with get, set, delete, list (available globally)",
		"console":  "Console logging with structured output (available globally)",
		"encoding": "TextEncoder/TextDecoder for UTF-8 encoding/decoding (available globally)",
		"url":      "URL parsing and URLSearchParams manipulation (available globally)",
	}

	// Add enabled modules with descriptions
	for _, module := range enabledModules {
		if desc, exists := moduleDescriptions[module]; exists {
			description.WriteString(fmt.Sprintf("• %s: %s\n", module, desc))
		}
	}

	// Add usage examples
	description.WriteString("\nExample usage (modern JavaScript with require()):\n")
	description.WriteString("```javascript\n")
	description.WriteString("// Basic JavaScript execution\n")
	description.WriteString("const result = 2 + 3;\n")
	description.WriteString("console.log('Result:', result);\n\n")

	// Create a set for faster lookup
	enabledSet := make(map[string]bool)
	for _, module := range enabledModules {
		enabledSet[module] = true
	}

	// Add examples only for enabled modules
	if enabledSet["fetch"] {
		description.WriteString("// Fetch API (available globally when enabled)\n")
		description.WriteString("const response = await fetch('https://api.example.com/data');\n")
		description.WriteString("const data = await response.json();\n")
		description.WriteString("console.log(data);\n\n")
	}

	if enabledSet["http"] {
		description.WriteString("// HTTP server (require import - NOT import statement)\n")
		description.WriteString("const serve = require('http/server');\n")
		description.WriteString("const server = serve(8000, async (req) => {\n")
		description.WriteString("  return new Response(`Hello ${req.method} ${req.url}!`);\n")
		description.WriteString("});\n")
		description.WriteString("console.log('Server running at:', server.url);\n\n")
	}

	if enabledSet["crypto"] {
		description.WriteString("// Crypto operations (require import)\n")
		description.WriteString("const crypto = require('crypto');\n")
		description.WriteString("const hash = crypto.md5('hello').hex();\n")
		description.WriteString("console.log('MD5 hash:', hash);\n\n")
	}

	if enabledSet["timers"] {
		description.WriteString("// Timers (available globally)\n")
		description.WriteString("setTimeout(() => {\n")
		description.WriteString("  console.log('Hello after 1 second');\n")
		description.WriteString("}, 1000);\n\n")
	}

	if enabledSet["buffer"] {
		description.WriteString("// Buffer operations (available globally)\n")
		description.WriteString("const buffer = Buffer.from('hello', 'utf8');\n")
		description.WriteString("console.log(buffer.toString('base64'));\n\n")
	}

	description.WriteString("```\n")
	description.WriteString("\nImportant notes:\n")
	description.WriteString("• Use require() for modules, NOT import statements\n")
	description.WriteString("• Modern JavaScript features supported (const/let, arrow functions, destructuring, etc.)\n")
	description.WriteString("• HTTP servers automatically run in background and don't block execution\n")
	description.WriteString("• Async/await and Promises are fully supported\n")

	return description.String()
}

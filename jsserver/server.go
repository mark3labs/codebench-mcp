package jsserver

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/grafana/sobek"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shiroyk/ski"
	"github.com/shiroyk/ski/js"

	// Import ski modules
	_ "github.com/shiroyk/ski/modules/buffer"
	_ "github.com/shiroyk/ski/modules/cache"
	_ "github.com/shiroyk/ski/modules/crypto"
	_ "github.com/shiroyk/ski/modules/dom"
	_ "github.com/shiroyk/ski/modules/encoding"
	_ "github.com/shiroyk/ski/modules/ext"
	_ "github.com/shiroyk/ski/modules/fetch"
	_ "github.com/shiroyk/ski/modules/html"
	_ "github.com/shiroyk/ski/modules/http"
	httpmodule "github.com/shiroyk/ski/modules/http"
	_ "github.com/shiroyk/ski/modules/signal"
	_ "github.com/shiroyk/ski/modules/stream"
	_ "github.com/shiroyk/ski/modules/timers"
	_ "github.com/shiroyk/ski/modules/url"
)

var Version = "dev"

// captureLogger captures log output to a buffer
type captureLogger struct {
	buffer *bytes.Buffer
}

func (c *captureLogger) Enabled(context.Context, slog.Level) bool {
	return true
}

func (c *captureLogger) Handle(ctx context.Context, record slog.Record) error {
	c.buffer.WriteString(record.Message)
	c.buffer.WriteString("\n")
	return nil
}

func (c *captureLogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	return c
}

func (c *captureLogger) WithGroup(name string) slog.Handler {
	return c
}

type ModuleConfig struct {
	EnabledModules  []string
	DisabledModules []string
}

type JSHandler struct {
	config ModuleConfig
}

func NewJSHandler() *JSHandler {
	return NewJSHandlerWithConfig(ModuleConfig{
		EnabledModules: []string{"http", "fetch", "timers", "buffer", "crypto"},
	})
}

func NewJSHandlerWithConfig(config ModuleConfig) *JSHandler {
	return &JSHandler{
		config: config,
	}
}

func (h *JSHandler) isModuleEnabled(module string) bool {
	// If disabled modules list is provided, check if module is not in it
	if len(h.config.DisabledModules) > 0 {
		for _, disabled := range h.config.DisabledModules {
			if disabled == module {
				return false
			}
		}
		return true
	}

	// Otherwise check enabled modules list
	if len(h.config.EnabledModules) == 0 {
		return true // If no config, enable all
	}

	for _, enabled := range h.config.EnabledModules {
		if enabled == module {
			return true
		}
	}
	return false
}

func (h *JSHandler) handleExecuteJS(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	code, err := request.RequireString("code")
	if err != nil {
		return nil, err
	}

	// Capture console output
	var output bytes.Buffer
	captureHandler := &captureLogger{buffer: &output}
	logger := slog.New(captureHandler)

	// Create context with custom logger
	ctx = js.WithLogger(ctx, logger)

	// Create a custom scheduler with limited modules based on config
	schedulerOpts := ski.SchedulerOptions{
		InitialVMs: 1,
		MaxVMs:     1,
	}

	// Set up the scheduler with filtered modules
	scheduler := ski.NewScheduler(schedulerOpts)
	ski.SetScheduler(scheduler)
	defer scheduler.Close()

	// Create a VM with proper module initialization
	vm := js.NewVM()

	// Override the HTTP server module to make it non-blocking if enabled
	if h.isModuleEnabled("http") {
		// Create a custom module loader that wraps the HTTP server
		vm.Runtime().Set("__originalRequire", vm.Runtime().Get("require"))
		vm.Runtime().Set("require", vm.Runtime().ToValue(func(call sobek.FunctionCall) sobek.Value {
			moduleName := call.Argument(0).String()
			
			// If requesting the HTTP server module, return our wrapped version
			if moduleName == "ski/http/server" {
				httpServer := &httpmodule.Server{}
				value, err := httpServer.Instantiate(vm.Runtime())
				if err != nil {
					panic(vm.Runtime().NewGoError(err))
				}
				
				// Wrap the serve function to automatically unref servers
				wrappedServe := vm.Runtime().ToValue(func(call sobek.FunctionCall) sobek.Value {
					// Call the original serve function
					serveFunc, ok := sobek.AssertFunction(value)
					if !ok {
						panic(vm.Runtime().NewTypeError("serve is not a function"))
					}
					
					result, err := serveFunc(sobek.Undefined(), call.Arguments...)
					if err != nil {
						panic(vm.Runtime().NewGoError(err))
					}
					
					// If the result is a server object, unref it to prevent blocking
					if server, ok := result.(*sobek.Object); ok {
						if unref := server.Get("unref"); unref != nil {
							if unrefFunc, ok := sobek.AssertFunction(unref); ok {
								_, _ = unrefFunc(server)
							}
						}
					}
					
					return result
				})
				
				return wrappedServe
			}
			
			// For all other modules, use the original require
			originalRequire, _ := sobek.AssertFunction(vm.Runtime().Get("__originalRequire"))
			result, err := originalRequire(sobek.Undefined(), call.Arguments...)
			if err != nil {
				panic(vm.Runtime().NewGoError(err))
			}
			return result
		}))
	}

	// Execute the JavaScript code using the VM with a timeout context
	// This allows servers to start but doesn't block indefinitely
	execCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	
	result, err := vm.RunString(execCtx, code)

	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("JavaScript execution error: %v\n\nOutput:\n%s", err, output.String()),
				},
			},
			IsError: true,
		}, nil
	}

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
				Text: fmt.Sprintf("%s%s", resultStr, output.String()),
			},
		},
	}, nil
}

func (h *JSHandler) getAvailableModules() []string {
	allModules := []string{
		"http", "fetch", "timers", "buffer", "cache", "crypto", "dom",
		"encoding", "ext", "html", "signal", "stream", "url",
	}

	// Always filter through isModuleEnabled for consistency
	var enabled []string
	for _, module := range allModules {
		if h.isModuleEnabled(module) {
			enabled = append(enabled, module)
		}
	}
	return enabled
}

func NewJSServer() (*server.MCPServer, error) {
	return NewJSServerWithConfig(ModuleConfig{
		EnabledModules: []string{"http", "fetch", "timers", "buffer", "crypto"},
	})
}

func NewJSServerWithConfig(config ModuleConfig) (*server.MCPServer, error) {
	h := NewJSHandlerWithConfig(config)

	s := server.NewMCPServer(
		"javascript-executor",
		Version,
	)

	// Build detailed description with module information
	description := buildToolDescription(h.getAvailableModules())

	// Register the executeJS tool
	s.AddTool(mcp.NewTool(
		"executeJS",
		mcp.WithDescription(description),
		mcp.WithString("code",
			mcp.Description("JavaScript code to execute with Node.js-like APIs"),
			mcp.Required(),
		),
	), h.handleExecuteJS)

	return s, nil
}

func buildToolDescription(enabledModules []string) string {
	var description strings.Builder

	description.WriteString("Execute JavaScript code with Node.js-like APIs powered by ski runtime. ")
	description.WriteString("Supports modern JavaScript (ES2020+), CommonJS modules via require(), promises, and comprehensive JavaScript APIs. ")
	description.WriteString("ES6 import statements are not supported in direct execution - use require() instead.\n\n")

	if len(enabledModules) == 0 {
		description.WriteString("No modules are currently enabled. Only basic JavaScript execution is available.")
		return description.String()
	}

	description.WriteString("Available modules:\n")

	// Define module descriptions with ski's actual features and require paths
	moduleDescriptions := map[string]string{
		"http":     "HTTP server creation and management (const serve = require('ski/http/server'))",
		"fetch":    "Modern fetch API with Request, Response, Headers, FormData (available globally)",
		"timers":   "setTimeout, setInterval, clearTimeout, clearInterval (available globally)",
		"buffer":   "Buffer, Blob, File APIs for binary data handling (available globally)",
		"cache":    "In-memory caching with TTL support (const cache = require('ski/cache'))",
		"crypto":   "Cryptographic functions (hashing, encryption, HMAC) (const crypto = require('ski/crypto'))",
		"dom":      "DOM Event and EventTarget APIs (const dom = require('ski/dom'))",
		"encoding": "TextEncoder, TextDecoder for text encoding/decoding (available globally)",
		"ext":      "Extended context and utility functions (const ext = require('ski/ext'))",
		"html":     "HTML parsing and manipulation (const html = require('ski/html'))",
		"signal":   "AbortController and AbortSignal for cancellation (available globally)",
		"stream":   "ReadableStream and streaming APIs (available globally)",
		"url":      "URL and URLSearchParams APIs (available globally)",
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
		description.WriteString("const serve = require('ski/http/server');\n")
		description.WriteString("const server = serve(8000, async (req) => {\n")
		description.WriteString("  return new Response(`Hello ${req.method} ${req.url}!`);\n")
		description.WriteString("});\n")
		description.WriteString("console.log('Server running at:', server.url);\n\n")
	}
	
	if enabledSet["cache"] {
		description.WriteString("// Cache operations (require import)\n")
		description.WriteString("const cache = require('ski/cache');\n")
		description.WriteString("cache.set('key', 'value');\n")
		description.WriteString("console.log(cache.get('key'));\n\n")
	}
	
	if enabledSet["crypto"] {
		description.WriteString("// Crypto operations (require import)\n")
		description.WriteString("const crypto = require('ski/crypto');\n")
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
	
	// Add HTTP-specific note only if HTTP is enabled
	if enabledSet["http"] {
		description.WriteString("• HTTP servers automatically run in background and don't block execution\n")
	}
	
	description.WriteString("• Async/await and Promises are fully supported\n")

	return description.String()
}

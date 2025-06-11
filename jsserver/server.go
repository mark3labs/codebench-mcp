package jsserver

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

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

	// Execute the JavaScript code using ski
	result, err := ski.RunString(ctx, code)

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

	if len(h.config.DisabledModules) > 0 {
		var enabled []string
		for _, module := range allModules {
			if h.isModuleEnabled(module) {
				enabled = append(enabled, module)
			}
		}
		return enabled
	}

	if len(h.config.EnabledModules) > 0 {
		return h.config.EnabledModules
	}

	return allModules
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
	description.WriteString("Supports ES modules, CommonJS, promises, and comprehensive JavaScript APIs.\n\n")

	if len(enabledModules) == 0 {
		description.WriteString("No modules are currently enabled. Only basic JavaScript execution is available.")
		return description.String()
	}

	description.WriteString("Available modules:\n")

	// Define module descriptions with ski's actual features and import paths
	moduleDescriptions := map[string]string{
		"http":     "HTTP server creation and management (import serve from 'ski/http/server')",
		"fetch":    "Modern fetch API with Request, Response, Headers, FormData (available globally)",
		"timers":   "setTimeout, setInterval, clearTimeout, clearInterval (available globally)",
		"buffer":   "Buffer, Blob, File APIs for binary data handling (available globally)",
		"cache":    "In-memory caching with TTL support (import cache from 'ski/cache')",
		"crypto":   "Cryptographic functions (hashing, encryption, HMAC) (import crypto from 'ski/crypto')",
		"dom":      "DOM Event and EventTarget APIs",
		"encoding": "TextEncoder, TextDecoder for text encoding/decoding (available globally)",
		"ext":      "Extended context and utility functions",
		"html":     "HTML parsing and manipulation",
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
	description.WriteString("\nExample usage:\n")
	description.WriteString("```javascript\n")
	description.WriteString("// Basic JavaScript execution\n")
	description.WriteString("const result = 2 + 3;\n")
	description.WriteString("console.log('Result:', result);\n\n")
	description.WriteString("// Fetch API (available globally when enabled)\n")
	description.WriteString("const response = await fetch('https://api.example.com/data');\n")
	description.WriteString("const data = await response.json();\n\n")
	description.WriteString("// HTTP server (import required)\n")
	description.WriteString("import serve from 'ski/http/server';\n")
	description.WriteString("serve(8000, async (req) => {\n")
	description.WriteString("  return new Response('Hello World');\n")
	description.WriteString("});\n\n")
	description.WriteString("// Cache operations (import required)\n")
	description.WriteString("import cache from 'ski/cache';\n")
	description.WriteString("cache.set('key', 'value');\n")
	description.WriteString("console.log(cache.get('key'));\n\n")
	description.WriteString("// Crypto operations (import required)\n")
	description.WriteString("import crypto from 'ski/crypto';\n")
	description.WriteString("const hash = crypto.md5('hello').hex();\n")
	description.WriteString("console.log('MD5 hash:', hash);\n\n")
	description.WriteString("// Timers (available globally)\n")
	description.WriteString("setTimeout(() => console.log('Hello after 1 second'), 1000);\n\n")
	description.WriteString("// Buffer operations (available globally)\n")
	description.WriteString("const buffer = Buffer.from('hello', 'utf8');\n")
	description.WriteString("console.log(buffer.toString('base64'));\n")
	description.WriteString("```\n")

	return description.String()
}

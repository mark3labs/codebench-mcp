package jsserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var Version = "dev"

type ModuleConfig struct {
	EnabledModules []string
}

type JSHandler struct {
	config ModuleConfig
}

func NewJSHandler() *JSHandler {
	return NewJSHandlerWithConfig(ModuleConfig{
		EnabledModules: []string{"console", "fs", "http", "timers", "process", "require"},
	})
}

func NewJSHandlerWithConfig(config ModuleConfig) *JSHandler {
	return &JSHandler{
		config: config,
	}
}

func (h *JSHandler) isModuleEnabled(module string) bool {
	return slices.Contains(h.config.EnabledModules, module)
}

func setupConsole(vm *goja.Runtime) {
	console := vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		for _, arg := range call.Arguments {
			fmt.Print(arg.Export(), " ")
		}
		fmt.Println()
		return goja.Undefined()
	})
	console.Set("error", func(call goja.FunctionCall) goja.Value {
		for _, arg := range call.Arguments {
			fmt.Print(arg.Export(), " ")
		}
		fmt.Println()
		return goja.Undefined()
	})
	console.Set("warn", func(call goja.FunctionCall) goja.Value {
		for _, arg := range call.Arguments {
			fmt.Print(arg.Export(), " ")
		}
		fmt.Println()
		return goja.Undefined()
	})
	vm.Set("console", console)
}

func setupTimers(vm *goja.Runtime) {
	var intervals = make(map[int]*time.Ticker)
	var timeouts = make(map[int]*time.Timer)
	var nextID = 1

	vm.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
		fn, _ := goja.AssertFunction(call.Arguments[0])
		delay := time.Duration(call.Arguments[1].ToInteger()) * time.Millisecond
		id := nextID
		nextID++
		timeouts[id] = time.AfterFunc(delay, func() {
			_, _ = fn(goja.Undefined())
		})
		return vm.ToValue(id)
	})

	vm.Set("clearTimeout", func(call goja.FunctionCall) goja.Value {
		id := int(call.Arguments[0].ToInteger())
		if t, ok := timeouts[id]; ok {
			t.Stop()
			delete(timeouts, id)
		}
		return goja.Undefined()
	})

	vm.Set("setInterval", func(call goja.FunctionCall) goja.Value {
		fn, _ := goja.AssertFunction(call.Arguments[0])
		delay := time.Duration(call.Arguments[1].ToInteger()) * time.Millisecond
		ticker := time.NewTicker(delay)
		id := nextID
		nextID++
		intervals[id] = ticker

		go func() {
			for range ticker.C {
				_, _ = fn(goja.Undefined())
			}
		}()

		return vm.ToValue(id)
	})

	vm.Set("clearInterval", func(call goja.FunctionCall) goja.Value {
		id := int(call.Arguments[0].ToInteger())
		if t, ok := intervals[id]; ok {
			t.Stop()
			delete(intervals, id)
		}
		return goja.Undefined()
	})
}

func setupFS(vm *goja.Runtime) {
	fs := vm.NewObject()
	fs.Set("readFileSync", func(call goja.FunctionCall) goja.Value {
		filename := call.Arguments[0].String()
		data, err := os.ReadFile(filename)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		return vm.ToValue(string(data))
	})
	fs.Set("writeFileSync", func(call goja.FunctionCall) goja.Value {
		filename := call.Arguments[0].String()
		content := call.Arguments[1].String()
		err := os.WriteFile(filename, []byte(content), 0644)
		if err != nil {
			panic(vm.ToValue(err.Error()))
		}
		return goja.Undefined()
	})
	fs.Set("existsSync", func(call goja.FunctionCall) goja.Value {
		filename := call.Arguments[0].String()
		_, err := os.Stat(filename)
		return vm.ToValue(err == nil)
	})
	vm.Set("fs", fs)
}

func setupHTTP(vm *goja.Runtime) {
	httpMod := vm.NewObject()
	httpMod.Set("createServer", func(call goja.FunctionCall) goja.Value {
		handler, _ := goja.AssertFunction(call.Arguments[0])
		
		server := vm.NewObject()
		var httpServer *http.Server
		
		server.Set("listen", func(call goja.FunctionCall) goja.Value {
			port := ":8080" // Default port
			if len(call.Arguments) > 0 && !goja.IsUndefined(call.Arguments[0]) {
				port = fmt.Sprintf(":%d", call.Arguments[0].ToInteger())
			}
			
			go func() {
				httpServer = &http.Server{
					Addr: port,
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						req := vm.NewObject()
						req.Set("url", r.URL.Path)
						req.Set("method", r.Method)

						res := vm.NewObject()
						res.Set("writeHead", func(call goja.FunctionCall) goja.Value {
							if len(call.Arguments) > 0 {
								statusCode := int(call.Arguments[0].ToInteger())
								w.WriteHeader(statusCode)
							}
							if len(call.Arguments) > 1 && !goja.IsUndefined(call.Arguments[1]) {
								headersObj := call.Arguments[1].(*goja.Object)
								for _, key := range headersObj.Keys() {
									w.Header().Set(key, headersObj.Get(key).String())
								}
							}
							return goja.Undefined()
						})
						
						res.Set("end", func(call goja.FunctionCall) goja.Value {
							if len(call.Arguments) > 0 {
								w.Write([]byte(call.Arguments[0].String()))
							}
							return goja.Undefined()
						})

						_, err := handler(goja.Undefined(), req, res)
						if err != nil {
							fmt.Println("HTTP handler error:", err)
						}
					}),
				}
				httpServer.ListenAndServe()
			}()
			return goja.Undefined()
		})
		
		server.Set("close", func(call goja.FunctionCall) goja.Value {
			if httpServer != nil {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					httpServer.Shutdown(ctx)
				}()
			}
			return goja.Undefined()
		})
		
		server.Set("address", func(call goja.FunctionCall) goja.Value {
			if httpServer != nil {
				addr := vm.NewObject()
				addr.Set("address", httpServer.Addr)
				return addr
			}
			return goja.Null()
		})
		
		return server
	})
	vm.Set("http", httpMod)
}

func setupProcess(vm *goja.Runtime) {
	proc := vm.NewObject()
	proc.Set("argv", os.Args)
	proc.Set("cwd", func(goja.FunctionCall) goja.Value {
		dir, _ := os.Getwd()
		return vm.ToValue(dir)
	})
	proc.Set("exit", func(call goja.FunctionCall) goja.Value {
		code := 0
		if len(call.Arguments) > 0 {
			code = int(call.Arguments[0].ToInteger())
		}
		os.Exit(code)
		return goja.Undefined()
	})

	env := vm.NewObject()
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			env.Set(pair[0], pair[1])
		}
	}
	proc.Set("env", env)

	vm.Set("process", proc)
}

func setupRequire(vm *goja.Runtime, basePath string, config ModuleConfig) {
	vm.Set("require", func(call goja.FunctionCall) goja.Value {
		modulePath := call.Arguments[0].String()

		// Handle built-in modules
		switch modulePath {
		case "fs":
			return vm.Get("fs")
		case "http":
			return vm.Get("http")
		case "process":
			return vm.Get("process")
		}

		// Handle file modules
		fullPath := filepath.Join(basePath, "modules", modulePath+".js")
		if !strings.HasSuffix(modulePath, ".js") {
			fullPath = filepath.Join(basePath, modulePath+".js")
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			panic(vm.ToValue("Module not found: " + fullPath))
		}

		newVM := goja.New()
		if slices.Contains(config.EnabledModules, "console") {
			setupConsole(newVM)
		}
		if slices.Contains(config.EnabledModules, "timers") {
			setupTimers(newVM)
		}
		if slices.Contains(config.EnabledModules, "fs") {
			setupFS(newVM)
		}
		if slices.Contains(config.EnabledModules, "http") {
			setupHTTP(newVM)
		}
		if slices.Contains(config.EnabledModules, "process") {
			setupProcess(newVM)
		}
		if slices.Contains(config.EnabledModules, "require") {
			setupRequire(newVM, basePath, config)
		}

		exports := newVM.NewObject()
		newVM.Set("exports", exports)
		newVM.Set("module", map[string]any{"exports": exports})

		_, err = newVM.RunString(string(data))
		if err != nil {
			panic(vm.ToValue("Error in module: " + err.Error()))
		}

		return newVM.Get("exports")
	})
}

func (h *JSHandler) handleExecuteJS(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	code, err := request.RequireString("code")
	if err != nil {
		return nil, err
	}

	// Create a new VM for each execution
	vm := goja.New()

	// Setup all the APIs conditionally
	if h.isModuleEnabled("console") {
		setupConsole(vm)
	}
	if h.isModuleEnabled("timers") {
		setupTimers(vm)
	}
	if h.isModuleEnabled("fs") {
		setupFS(vm)
	}
	if h.isModuleEnabled("http") {
		setupHTTP(vm)
	}
	if h.isModuleEnabled("process") {
		setupProcess(vm)
	}
	if h.isModuleEnabled("require") {
		setupRequire(vm, ".", h.config)
	}

	// Capture output
	var output strings.Builder

	// Override console.log to capture output only if console module is enabled
	if h.isModuleEnabled("console") {
		console := vm.NewObject()
		console.Set("log", func(call goja.FunctionCall) goja.Value {
			for i, arg := range call.Arguments {
				if i > 0 {
					output.WriteString(" ")
				}
				output.WriteString(fmt.Sprintf("%v", arg.Export()))
			}
			output.WriteString("\n")
			return goja.Undefined()
		})
		console.Set("error", func(call goja.FunctionCall) goja.Value {
			output.WriteString("ERROR: ")
			for i, arg := range call.Arguments {
				if i > 0 {
					output.WriteString(" ")
				}
				output.WriteString(fmt.Sprintf("%v", arg.Export()))
			}
			output.WriteString("\n")
			return goja.Undefined()
		})
		console.Set("warn", func(call goja.FunctionCall) goja.Value {
			output.WriteString("WARN: ")
			for i, arg := range call.Arguments {
				if i > 0 {
					output.WriteString(" ")
				}
				output.WriteString(fmt.Sprintf("%v", arg.Export()))
			}
			output.WriteString("\n")
			return goja.Undefined()
		})
		vm.Set("console", console)
	}

	// Execute the JavaScript code
	result, err := vm.RunString(code)
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
	if result != nil && !goja.IsUndefined(result) && !goja.IsNull(result) {
		resultStr = fmt.Sprintf("Result: %v\n", result.Export())
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

func NewJSServer() (*server.MCPServer, error) {
	return NewJSServerWithConfig(ModuleConfig{
		EnabledModules: []string{"console", "fs", "http", "timers", "process", "require"},
	})
}

func NewJSServerWithConfig(config ModuleConfig) (*server.MCPServer, error) {
	h := NewJSHandlerWithConfig(config)

	s := server.NewMCPServer(
		"javascript-executor",
		Version,
	)

	// Build detailed description with module information
	description := buildToolDescription(config.EnabledModules)

	// Register the executeJS tool
	s.AddTool(mcp.NewTool(
		"executeJS",
		mcp.WithDescription(description),
		mcp.WithString("code",
			mcp.Description("JavaScript code to execute in the simplified VM"),
			mcp.Required(),
		),
	), h.handleExecuteJS)

	return s, nil
}

func buildToolDescription(enabledModules []string) string {
	var description strings.Builder

	description.WriteString("Execute JavaScript code in a simplified JavaScript VM (goja). ")
	description.WriteString("This is NOT a full Node.js environment - only the modules listed below are available.\n\n")

	if len(enabledModules) == 0 {
		description.WriteString("⚠️  No modules are currently enabled. Only basic JavaScript execution is available.")
		return description.String()
	}

	description.WriteString("📦 Available modules:\n")

	// Define module descriptions
	moduleDescriptions := map[string]string{
		"console": "Console logging (console.log, console.error, console.warn)",
		"fs":      "File system operations (fs.readFileSync, fs.writeFileSync, fs.existsSync)",
		"http":    "HTTP server creation (http.createServer with configurable ports and callbacks)",
		"timers":  "Timer functions (setTimeout, setInterval, clearTimeout, clearInterval)",
		"process": "Process information (process.argv, process.cwd, process.env, process.exit)",
		"require": "Module loading system (require() for loading JavaScript modules)",
	}

	// Add enabled modules with descriptions
	for _, module := range enabledModules {
		if desc, exists := moduleDescriptions[module]; exists {
			description.WriteString(fmt.Sprintf("• %s: %s\n", module, desc))
		}
	}

	// Add usage note
	description.WriteString("\n💡 Usage: Provide JavaScript code that uses only the enabled modules above. ")
	description.WriteString("Attempting to use disabled modules will result in 'undefined' errors.")

	return description.String()
}

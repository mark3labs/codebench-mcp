package jsserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var Version = "dev"

type JSHandler struct {
	mu        sync.Mutex
	intervals map[int]*time.Ticker
	timeouts  map[int]*time.Timer
	nextID    int
}

func NewJSHandler() *JSHandler {
	return &JSHandler{
		intervals: make(map[int]*time.Ticker),
		timeouts:  make(map[int]*time.Timer),
		nextID:    1,
	}
}

func (h *JSHandler) setupConsole(vm *goja.Runtime) {
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

func (h *JSHandler) setupTimers(vm *goja.Runtime) {
	vm.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
		fn, _ := goja.AssertFunction(call.Arguments[0])
		delay := time.Duration(call.Arguments[1].ToInteger()) * time.Millisecond

		h.mu.Lock()
		id := h.nextID
		h.nextID++
		h.mu.Unlock()

		h.timeouts[id] = time.AfterFunc(delay, func() {
			_, _ = fn(goja.Undefined())
			h.mu.Lock()
			delete(h.timeouts, id)
			h.mu.Unlock()
		})
		return vm.ToValue(id)
	})

	vm.Set("clearTimeout", func(call goja.FunctionCall) goja.Value {
		id := int(call.Arguments[0].ToInteger())
		h.mu.Lock()
		if t, ok := h.timeouts[id]; ok {
			t.Stop()
			delete(h.timeouts, id)
		}
		h.mu.Unlock()
		return goja.Undefined()
	})

	vm.Set("setInterval", func(call goja.FunctionCall) goja.Value {
		fn, _ := goja.AssertFunction(call.Arguments[0])
		delay := time.Duration(call.Arguments[1].ToInteger()) * time.Millisecond
		ticker := time.NewTicker(delay)

		h.mu.Lock()
		id := h.nextID
		h.nextID++
		h.intervals[id] = ticker
		h.mu.Unlock()

		go func() {
			for range ticker.C {
				_, _ = fn(goja.Undefined())
			}
		}()

		return vm.ToValue(id)
	})

	vm.Set("clearInterval", func(call goja.FunctionCall) goja.Value {
		id := int(call.Arguments[0].ToInteger())
		h.mu.Lock()
		if t, ok := h.intervals[id]; ok {
			t.Stop()
			delete(h.intervals, id)
		}
		h.mu.Unlock()
		return goja.Undefined()
	})
}

func (h *JSHandler) setupFS(vm *goja.Runtime) {
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

func (h *JSHandler) setupHTTP(vm *goja.Runtime) {
	httpMod := vm.NewObject()
	httpMod.Set("createServer", func(call goja.FunctionCall) goja.Value {
		handler, _ := goja.AssertFunction(call.Arguments[0])

		server := vm.NewObject()
		server.Set("listen", func(call goja.FunctionCall) goja.Value {
			port := ":8080"
			if len(call.Arguments) > 0 {
				port = ":" + call.Arguments[0].String()
			}

			go func() {
				http.ListenAndServe(port, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					req := vm.NewObject()
					req.Set("url", r.URL.Path)
					req.Set("method", r.Method)

					headers := vm.NewObject()
					for k, v := range r.Header {
						if len(v) > 0 {
							headers.Set(k, v[0])
						}
					}
					req.Set("headers", headers)

					res := vm.NewObject()
					res.Set("end", func(call goja.FunctionCall) goja.Value {
						if len(call.Arguments) > 0 {
							w.Write([]byte(call.Arguments[0].String()))
						}
						return goja.Undefined()
					})
					res.Set("writeHead", func(call goja.FunctionCall) goja.Value {
						if len(call.Arguments) > 0 {
							w.WriteHeader(int(call.Arguments[0].ToInteger()))
						}
						return goja.Undefined()
					})

					_, err := handler(goja.Undefined(), req, res)
					if err != nil {
						fmt.Println("HTTP handler error:", err)
					}
				}))
			}()
			return goja.Undefined()
		})

		return server
	})
	vm.Set("http", httpMod)
}

func (h *JSHandler) setupProcess(vm *goja.Runtime) {
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

func (h *JSHandler) setupRequire(vm *goja.Runtime, basePath string) {
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
		h.setupConsole(newVM)
		h.setupTimers(newVM)
		h.setupFS(newVM)
		h.setupHTTP(newVM)
		h.setupProcess(newVM)
		h.setupRequire(newVM, basePath)

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

	// Setup all the APIs
	h.setupConsole(vm)
	h.setupTimers(vm)
	h.setupFS(vm)
	h.setupHTTP(vm)
	h.setupProcess(vm)
	h.setupRequire(vm, ".")

	// Capture output
	var output strings.Builder

	// Override console.log to capture output
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
	h := NewJSHandler()

	s := server.NewMCPServer(
		"javascript-executor",
		Version,
	)

	// Register the executeJS tool
	s.AddTool(mcp.NewTool(
		"executeJS",
		mcp.WithDescription("Execute JavaScript code with Node.js-like APIs including console, fs, http, timers, and require."),
		mcp.WithString("code",
			mcp.Description("JavaScript code to execute"),
			mcp.Required(),
		),
	), h.handleExecuteJS)

	return s, nil
}

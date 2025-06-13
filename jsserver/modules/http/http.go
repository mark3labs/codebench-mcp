package http

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/internal/logger"
	"github.com/mark3labs/codebench-mcp/jsserver/vm"
)

// HTTPModule provides HTTP server functionality
type HTTPModule struct{}

// NewHTTPModule creates a new HTTP module
func NewHTTPModule() *HTTPModule {
	return &HTTPModule{}
}

// Name returns the module name
func (h *HTTPModule) Name() string {
	return "http"
}

// httpServer represents a running HTTP server instance
type httpServer struct {
	runtime  *sobek.Runtime
	server   *http.Server
	hostname string
	port     int
	handler  sobek.Callable
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	running  bool
}

// Setup initializes the HTTP module in the VM
func (h *HTTPModule) Setup(runtime *sobek.Runtime, manager *vm.VMManager) error {
	// Create a require function that can load the HTTP server
	runtime.Set("require", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			return sobek.Undefined()
		}

		moduleName := call.Argument(0).String()

		switch moduleName {
		case "ski/http/server":
			// Return the serve function
			return runtime.ToValue(func(call sobek.FunctionCall) sobek.Value {
				return h.createServer(call, runtime)
			})
		default:
			// For other modules, return undefined
			return sobek.Undefined()
		}
	})

	return nil
}

// createServer creates and starts an HTTP server
func (h *HTTPModule) createServer(call sobek.FunctionCall, runtime *sobek.Runtime) sobek.Value {
	if len(call.Arguments) == 0 {
		panic(runtime.NewTypeError("serve requires at least one argument"))
	}

	// Default configuration
	port := 8000
	hostname := "127.0.0.1"
	var handler sobek.Callable

	// Parse arguments
	arg0 := call.Argument(0)
	if sobek.IsNumber(arg0) {
		// serve(port, handler)
		port = int(arg0.ToInteger())
		if len(call.Arguments) > 1 {
			var ok bool
			handler, ok = sobek.AssertFunction(call.Argument(1))
			if !ok {
				panic(runtime.NewTypeError("handler must be a function"))
			}
		}
	} else {
		// serve(handler) or serve(options, handler)
		var ok bool
		handler, ok = sobek.AssertFunction(arg0)
		if !ok {
			panic(runtime.NewTypeError("handler must be a function"))
		}
	}

	if handler == nil {
		panic(runtime.NewTypeError("handler is required"))
	}

	// Create server context
	ctx, cancel := context.WithCancel(context.Background())

	server := &httpServer{
		runtime:  runtime,
		hostname: hostname,
		port:     port,
		handler:  handler,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", hostname, port)
	server.server = &http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(server.handleRequest),
	}

	// Start server in goroutine
	go func() {
		server.mu.Lock()
		server.running = true
		server.mu.Unlock()

		logger.Debug("Starting HTTP server", "addr", addr)

		if err := server.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}

		server.mu.Lock()
		server.running = false
		server.mu.Unlock()
	}()

	// Create server object to return
	serverObj := runtime.NewObject()

	// Add properties
	serverObj.Set("url", fmt.Sprintf("http://%s:%d", hostname, port))
	serverObj.Set("port", port)
	serverObj.Set("hostname", hostname)

	// Add methods
	serverObj.Set("close", func(call sobek.FunctionCall) sobek.Value {
		server.shutdown()
		return sobek.Undefined()
	})

	serverObj.Set("shutdown", func(call sobek.FunctionCall) sobek.Value {
		server.shutdown()
		return sobek.Undefined()
	})

	// Store server reference for cleanup
	runtime.Set("__http_server__", server)

	return serverObj
}

// handleRequest handles incoming HTTP requests
func (s *httpServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Create request object for JavaScript
	reqObj := s.runtime.NewObject()
	reqObj.Set("method", r.Method)
	reqObj.Set("url", r.URL.Path)
	reqObj.Set("path", r.URL.Path)

	// Headers
	headersObj := s.runtime.NewObject()
	for key, values := range r.Header {
		if len(values) > 0 {
			headersObj.Set(key, values[0])
		}
	}
	reqObj.Set("headers", headersObj)

	// Call the JavaScript handler
	result, err := s.handler(sobek.Undefined(), s.runtime.ToValue(reqObj))
	if err != nil {
		logger.Error("Handler error", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Process the response
	if result != nil && !sobek.IsUndefined(result) {
		responseObj := result.ToObject(s.runtime)

		// Get status code
		status := 200
		if statusVal := responseObj.Get("status"); statusVal != nil && !sobek.IsUndefined(statusVal) {
			status = int(statusVal.ToInteger())
		}

		// Get headers
		if headersVal := responseObj.Get("headers"); headersVal != nil && !sobek.IsUndefined(headersVal) {
			headersObj := headersVal.ToObject(s.runtime)
			for _, key := range headersObj.Keys() {
				value := headersObj.Get(key).String()
				w.Header().Set(key, value)
			}
		}

		// Get body
		body := ""
		if bodyVal := responseObj.Get("body"); bodyVal != nil && !sobek.IsUndefined(bodyVal) {
			body = bodyVal.String()
		}

		// Write response
		w.WriteHeader(status)
		w.Write([]byte(body))
	} else {
		// Default response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// shutdown gracefully shuts down the server
func (s *httpServer) shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running && s.server != nil {
		logger.Debug("Shutting down HTTP server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.server.Shutdown(ctx); err != nil {
			logger.Error("Server shutdown error", "error", err)
		}
		s.running = false
	}

	if s.cancel != nil {
		s.cancel()
	}
}

// Cleanup performs any necessary cleanup
func (h *HTTPModule) Cleanup() error {
	// HTTP module cleanup is handled by individual server instances
	return nil
}

// IsEnabled checks if the module should be enabled based on configuration
func (h *HTTPModule) IsEnabled(enabledModules map[string]bool) bool {
	enabled, exists := enabledModules["http"]
	return exists && enabled
}

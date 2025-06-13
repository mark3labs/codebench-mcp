package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/internal/logger"
	"github.com/mark3labs/codebench-mcp/server/vm"
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

// Setup initializes the HTTP module in the VM
func (h *HTTPModule) Setup(runtime *sobek.Runtime, manager *vm.VMManager) error {
	// No setup needed - the module will be available via require()
	return nil
}

// CreateModuleObject creates the HTTP server module when required
func (h *HTTPModule) CreateModuleObject(runtime *sobek.Runtime) sobek.Value {
	// Return the serve function directly for http/server
	return runtime.ToValue(func(call sobek.FunctionCall) sobek.Value {
		return h.createServer(call, runtime)
	})
}

// createServer creates and starts an HTTP server
func (h *HTTPModule) createServer(call sobek.FunctionCall, runtime *sobek.Runtime) sobek.Value {
	serv := &httpServer{
		rt:       runtime,
		port:     8000,
		hostname: "127.0.0.1",
		ctx:      context.Background(),
		server:   &http.Server{Addr: "127.0.0.1:8000"},
	}

	if len(call.Arguments) == 0 {
		panic(runtime.NewTypeError("serve requires at least one argument"))
	}

	var handler sobek.Value

	opt := call.Argument(0)
	switch {
	case isNumber(opt):
		port := opt.ToInteger()
		if port <= 0 {
			panic(runtime.NewTypeError("port must be a positive number"))
		}
		serv.port = int(port)
		serv.server.Addr = fmt.Sprintf(":%d", serv.port)
		handler = call.Argument(1)
	case isFunc(opt):
		handler = opt
	default:
		opts := opt.ToObject(runtime)
		if v := opts.Get("port"); v != nil {
			serv.port = int(v.ToInteger())
			serv.server.Addr = fmt.Sprintf(":%d", serv.port)
		}
		if v := opts.Get("hostname"); v != nil {
			serv.hostname = v.String()
			serv.server.Addr = fmt.Sprintf("%s:%d", serv.hostname, serv.port)
		}
		if v := opts.Get("maxHeaderSize"); v != nil {
			serv.server.MaxHeaderBytes = int(v.ToInteger())
		}
		if v := opts.Get("keepAliveTimeout"); v != nil {
			serv.server.IdleTimeout = time.Duration(v.ToInteger()) * time.Millisecond
		}
		if v := opts.Get("requestTimeout"); v != nil {
			serv.server.ReadTimeout = time.Duration(v.ToInteger()) * time.Millisecond
		}
		if v := opts.Get("onError"); v != nil {
			var ok bool
			serv.onError, ok = sobek.AssertFunction(v)
			if !ok {
				panic(runtime.NewTypeError("onError must be a function"))
			}
		}
		if v := opts.Get("onListen"); v != nil {
			var ok bool
			serv.onListen, ok = sobek.AssertFunction(v)
			if !ok {
				panic(runtime.NewTypeError("onListen must be a function"))
			}
		}
		if v := opts.Get("handler"); v != nil {
			handler = v
		}
		if v := call.Argument(1); !sobek.IsUndefined(v) {
			handler = v
		}
	}

	if handler != nil {
		var ok bool
		serv.handler, ok = sobek.AssertFunction(handler)
		if !ok {
			panic(runtime.NewTypeError("handler must be a function"))
		}
	}
	if serv.onError == nil {
		serv.onError = func(this sobek.Value, args ...sobek.Value) (sobek.Value, error) {
			code := http.StatusInternalServerError
			msg := http.StatusText(code)
			if len(args) > 0 {
				err := args[0].ToObject(runtime)
				url := err.Get("url").String()
				method := err.Get("method").String()
				message := err.Get("message").String()
				msg = fmt.Sprintf("Internal Server Error %s %s %s", method, url, message)
			}
			logger.Error(msg)
			return newResponse(runtime, &http.Response{
				StatusCode: code,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(http.StatusText(code))),
			}), nil
		}
	}
	if serv.handler == nil {
		serv.handler = func(this sobek.Value, args ...sobek.Value) (sobek.Value, error) {
			code := http.StatusNotFound
			body := strings.NewReader(http.StatusText(http.StatusNotFound))
			return newResponse(runtime, &http.Response{
				StatusCode: code,
				Header:     make(http.Header),
				Body:       io.NopCloser(body),
			}), nil
		}
	}

	serv.server.Handler = serv
	serv.ref = vm.EnqueueJob(runtime)
	ln := serv.listen()

	go func() {
		vm.EnqueueJob(runtime)(func() error {
			if serv.onListen != nil {
				_, _ = serv.onListen(sobek.Undefined(), serv.addr())
			} else {
				logger.Info(fmt.Sprintf("listening on %s", serv.url()))
			}
			return nil
		})
		err := serv.server.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			vm.EnqueueJob(runtime)(func() error { return err })
		}
	}()

	// Create server object to return
	serverObj := runtime.NewObject()

	// Add properties
	serverObj.Set("url", serv.url())
	serverObj.Set("port", serv.port)
	serverObj.Set("hostname", serv.hostname)

	// Add methods
	serverObj.Set("close", func(call sobek.FunctionCall) sobek.Value {
		if err := serv.close(); err != nil {
			panic(runtime.NewGoError(err))
		}
		return sobek.Undefined()
	})

	serverObj.Set("shutdown", func(call sobek.FunctionCall) sobek.Value {
		if err := serv.shutdown(); err != nil {
			panic(runtime.NewGoError(err))
		}
		return sobek.Undefined()
	})

	return serverObj
}

type httpServer struct {
	rt       *sobek.Runtime
	server   *http.Server
	hostname string
	port     int

	handler, onError, onListen sobek.Callable

	ctx    context.Context
	closed atomic.Bool

	ref func(func() error)
}

func (s *httpServer) url() string {
	if s.port == 80 {
		return "http://" + s.hostname
	}
	return fmt.Sprintf("http://%s:%d", s.hostname, s.port)
}

func (s *httpServer) addr() sobek.Value {
	return s.rt.ToValue(map[string]any{
		"hostname": s.hostname,
		"port":     s.port,
	})
}

func (s *httpServer) listen() net.Listener {
	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		panic(s.rt.NewGoError(err))
	}
	return ln
}

func (s *httpServer) close() error {
	s.closed.Store(true)
	err := s.server.Close()
	if s.ref != nil {
		s.ref(func() error { s.ref = nil; return nil })
	}
	return err
}

func (s *httpServer) shutdown() error {
	s.closed.Store(true)
	err := s.server.Shutdown(s.ctx)
	if s.ref != nil {
		s.ref(func() error { s.ref = nil; return nil })
	}
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// ServeHTTP implements http.Handler
func (s *httpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	wg.Add(1)
	vm.EnqueueJob(s.rt)(func() error {
		result, err := s.handler(sobek.Undefined(), newRequest(s.rt, r))
		if err != nil {
			s.writeError(w, r, wg.Done, err)
			return nil
		}

		// Handle promise result
		if isPromise(result) {
			s.handlePromise(w, r, wg.Done, result)
			return nil
		}

		if res, ok := toResponse(result); ok {
			s.writeResponse(w, r, wg.Done, res)
		} else {
			s.writeError(w, r, wg.Done, errNotResponse)
		}
		return nil
	})
	wg.Wait()
}

func (s *httpServer) writeResponse(w http.ResponseWriter, r *http.Request, done func(), res *http.Response) {
	defer done()

	header := w.Header()
	for k, v := range res.Header {
		header[http.CanonicalHeaderKey(k)] = v
	}
	w.WriteHeader(res.StatusCode)

	if _, err := io.Copy(w, res.Body); err != nil {
		logger.Error("Failed to write response", "error", err, "method", r.Method, "url", r.URL.String())
	}
}

func (s *httpServer) writeError(w http.ResponseWriter, r *http.Request, done func(), rawErr error) {
	var (
		jsErr  *sobek.Object
		result sobek.Value
		err    error
	)

	ex, ok := rawErr.(*sobek.Exception)
	if ok {
		jsErr, ok = ex.Value().(*sobek.Object)
	}
	if !ok {
		jsErr, err = s.rt.New(s.rt.Get("Error"), s.rt.ToValue(rawErr.Error()))
		if err != nil {
			goto EX
		}
	}
	_ = jsErr.Set("method", r.Method)
	_ = jsErr.Set("headers", r.Header)
	_ = jsErr.Set("url", r.URL.String())

	result, err = s.onError(sobek.Undefined(), jsErr)
	if err != nil {
		goto EX
	}

	if !isPromise(result) {
		if res, ok := toResponse(result); ok {
			s.writeResponse(w, r, done, res)
			return
		}
		err = errNotResponse
	} else {
		switch p := result.Export().(*sobek.Promise); p.State() {
		case sobek.PromiseStateRejected:
			if ex, ok := p.Result().Export().(error); ok {
				err = ex
			} else {
				err = errors.New(p.Result().String())
			}
		case sobek.PromiseStateFulfilled:
			if res, ok := toResponse(result); ok {
				s.writeResponse(w, r, done, res)
				return
			}
			err = errNotResponse
		default:
			if err = s.handlePendingPromise(w, r, done, result); err == nil {
				return
			}
		}
	}

EX:
	logger.Error("Exception in onError while handling exception", "message", err.Error())
	w.WriteHeader(http.StatusInternalServerError)
	w.Write(internalServerError)
	done()
}

// handlePromise handles promise result
func (s *httpServer) handlePromise(w http.ResponseWriter, r *http.Request, done func(), result sobek.Value) {
	var err error
	switch p := result.Export().(*sobek.Promise); p.State() {
	case sobek.PromiseStateRejected:
		if ex, ok := p.Result().Export().(error); ok {
			err = ex
		} else {
			err = errors.New(p.Result().String())
		}
	case sobek.PromiseStateFulfilled:
		if res, ok := toResponse(p.Result()); ok {
			s.writeResponse(w, r, done, res)
		} else {
			err = errNotResponse
		}
	default:
		err = s.handlePendingPromise(w, r, done, result)
	}
	if err != nil {
		s.writeError(w, r, done, err)
	}
}

// handlePendingPromise handles a pending promise with resolve and reject callbacks
func (s *httpServer) handlePendingPromise(w http.ResponseWriter, r *http.Request, done func(), promise sobek.Value) error {
	object := promise.(*sobek.Object)
	then, ok := sobek.AssertFunction(object.Get("then"))
	if !ok {
		return errNotResponse
	}

	resolve := s.rt.ToValue(func(call sobek.FunctionCall) sobek.Value {
		if res, ok := toResponse(call.Argument(0)); ok {
			s.writeResponse(w, r, done, res)
		} else {
			s.writeError(w, r, done, errNotResponse)
		}
		return sobek.Undefined()
	})

	reject := s.rt.ToValue(func(call sobek.FunctionCall) sobek.Value {
		v := call.Argument(0)
		if ex, ok := v.Export().(error); ok {
			s.writeError(w, r, done, ex)
		} else {
			s.writeError(w, r, done, errors.New(v.String()))
		}
		return sobek.Undefined()
	})

	_, err := then(object, resolve, reject)
	return err
}

// Helper functions

func isNumber(v sobek.Value) bool {
	return sobek.IsNumber(v)
}

func isFunc(v sobek.Value) bool {
	_, ok := sobek.AssertFunction(v)
	return ok
}

func isPromise(value sobek.Value) bool {
	if obj := value.ToObject(nil); obj != nil {
		if thenMethod := obj.Get("then"); thenMethod != nil && !sobek.IsUndefined(thenMethod) {
			_, ok := sobek.AssertFunction(thenMethod)
			return ok
		}
	}
	return false
}

// newRequest creates a JavaScript request object from http.Request
func newRequest(runtime *sobek.Runtime, r *http.Request) sobek.Value {
	reqObj := runtime.NewObject()
	reqObj.Set("method", r.Method)
	reqObj.Set("url", r.URL.Path)
	reqObj.Set("path", r.URL.Path)

	// Headers
	headersObj := runtime.NewObject()
	for key, values := range r.Header {
		if len(values) > 0 {
			headersObj.Set(key, values[0])
		}
	}
	reqObj.Set("headers", headersObj)

	// Read request body
	bodyStr := ""
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			bodyStr = string(bodyBytes)
		}
		// Close the original body and replace with a new reader for downstream use
		r.Body.Close()
		r.Body = io.NopCloser(strings.NewReader(bodyStr))
	}
	
	reqObj.Set("body", bodyStr)
	
	// Add text() method for compatibility
	reqObj.Set("text", func(call sobek.FunctionCall) sobek.Value {
		return runtime.ToValue(bodyStr)
	})
	
	// Add json() method for convenience
	reqObj.Set("json", func(call sobek.FunctionCall) sobek.Value {
		if bodyStr == "" {
			return sobek.Null()
		}
		jsonVal, err := runtime.RunString("JSON.parse(" + runtime.ToValue(bodyStr).String() + ")")
		if err != nil {
			panic(runtime.NewGoError(err))
		}
		return jsonVal
	})

	return reqObj
}

// newResponse creates a Response object from http.Response
func newResponse(runtime *sobek.Runtime, resp *http.Response) sobek.Value {
	responseObj := runtime.NewObject()
	responseObj.Set("status", resp.StatusCode)
	responseObj.Set("statusText", resp.Status)
	responseObj.Set("ok", resp.StatusCode >= 200 && resp.StatusCode < 300)

	// Headers object
	headersObj := runtime.NewObject()
	for key, values := range resp.Header {
		if len(values) > 0 {
			headersObj.Set(key, values[0])
		}
	}
	responseObj.Set("headers", headersObj)

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(runtime.NewGoError(err))
	}
	resp.Body.Close()

	// text() method
	responseObj.Set("text", func(call sobek.FunctionCall) sobek.Value {
		return runtime.ToValue(string(bodyBytes))
	})

	// Store the actual http.Response for internal use
	responseObj.Set("__httpResponse", resp)

	return responseObj
}

// toResponse converts a sobek.Value to *http.Response
func toResponse(value sobek.Value) (*http.Response, bool) {
	if obj := value.ToObject(nil); obj != nil {
		// Check if it's our internal response object
		if httpResp := obj.Get("__httpResponse"); httpResp != nil && !sobek.IsUndefined(httpResp) {
			if resp, ok := httpResp.Export().(*http.Response); ok {
				return resp, true
			}
		}

		// Handle standard Response objects
		status := 200
		if statusVal := obj.Get("status"); statusVal != nil && !sobek.IsUndefined(statusVal) {
			status = int(statusVal.ToInteger())
		}

		headers := make(http.Header)
		if headersVal := obj.Get("headers"); headersVal != nil && !sobek.IsUndefined(headersVal) {
			headersObj := headersVal.ToObject(nil)
			for _, key := range headersObj.Keys() {
				value := headersObj.Get(key).String()
				headers.Set(key, value)
			}
		}

		// Get body content
		body := ""
		if bodyVal := obj.Get("body"); bodyVal != nil && !sobek.IsUndefined(bodyVal) {
			body = bodyVal.String()
		} else if textMethod := obj.Get("text"); textMethod != nil && !sobek.IsUndefined(textMethod) {
			if textFunc, ok := sobek.AssertFunction(textMethod); ok {
				textResult, err := textFunc(obj)
				if err == nil && textResult != nil && !sobek.IsUndefined(textResult) {
					body = textResult.String()
				}
			}
		}

		return &http.Response{
			StatusCode: status,
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(body)),
		}, true
	}
	return nil, false
}

var (
	internalServerError = []byte(http.StatusText(http.StatusInternalServerError))
	errNotResponse      = errors.New("return value from handler must be a response or a promise resolving to a response")
)

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
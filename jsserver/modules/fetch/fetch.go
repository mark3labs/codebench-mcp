package fetch

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/jsserver/vm"
)

// FetchModule provides fetch API functionality
type FetchModule struct {
	client *http.Client
}

// NewFetchModule creates a new fetch module
func NewFetchModule() *FetchModule {
	return &FetchModule{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the module name
func (f *FetchModule) Name() string {
	return "fetch"
}

// Setup initializes the fetch module in the VM
func (f *FetchModule) Setup(runtime *sobek.Runtime, manager *vm.VMManager) error {
	// fetch(url, options)
	runtime.Set("fetch", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			panic(runtime.NewTypeError("fetch: URL is required"))
		}

		url := call.Argument(0).String()

		// Default options
		method := "GET"
		var body io.Reader
		headers := make(map[string]string)

		// Parse options if provided
		if len(call.Arguments) > 1 && !sobek.IsUndefined(call.Argument(1)) {
			options := call.Argument(1).ToObject(runtime)

			if methodVal := options.Get("method"); methodVal != nil && !sobek.IsUndefined(methodVal) {
				method = strings.ToUpper(methodVal.String())
			}

			if bodyVal := options.Get("body"); bodyVal != nil && !sobek.IsUndefined(bodyVal) {
				bodyStr := bodyVal.String()
				body = strings.NewReader(bodyStr)
			}

			if headersVal := options.Get("headers"); headersVal != nil && !sobek.IsUndefined(headersVal) {
				headersObj := headersVal.ToObject(runtime)
				for _, key := range headersObj.Keys() {
					headers[key] = headersObj.Get(key).String()
				}
			}
		}

		// Create HTTP request
		req, err := http.NewRequest(method, url, body)
		if err != nil {
			panic(runtime.NewGoError(err))
		}

		// Set headers
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		// Make the request
		resp, err := f.client.Do(req)
		if err != nil {
			panic(runtime.NewGoError(err))
		}

		// Create Response object
		responseObj := runtime.NewObject()
		responseObj.Set("status", resp.StatusCode)
		responseObj.Set("statusText", resp.Status)
		responseObj.Set("ok", resp.StatusCode >= 200 && resp.StatusCode < 300)
		responseObj.Set("url", resp.Request.URL.String())

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
		resp.Body.Close()
		if err != nil {
			panic(runtime.NewGoError(err))
		}

		// text() method
		responseObj.Set("text", func(call sobek.FunctionCall) sobek.Value {
			return runtime.ToValue(string(bodyBytes))
		})

		// json() method
		responseObj.Set("json", func(call sobek.FunctionCall) sobek.Value {
			var result interface{}
			if err := runtime.ExportTo(runtime.ToValue(string(bodyBytes)), &result); err != nil {
				// Try to parse as JSON
				jsonVal, err := runtime.RunString("JSON.parse(" + runtime.ToValue(string(bodyBytes)).String() + ")")
				if err != nil {
					panic(runtime.NewGoError(err))
				}
				return jsonVal
			}
			return runtime.ToValue(result)
		})

		// arrayBuffer() method
		responseObj.Set("arrayBuffer", func(call sobek.FunctionCall) sobek.Value {
			return runtime.ToValue(bodyBytes)
		})

		return responseObj
	})

	// Request constructor
	runtime.Set("Request", func(call sobek.ConstructorCall) *sobek.Object {
		obj := call.This
		if len(call.Arguments) > 0 {
			obj.Set("url", call.Argument(0).String())
		}
		if len(call.Arguments) > 1 {
			obj.Set("options", call.Argument(1))
		}
		return nil
	})

	// Response constructor
	runtime.Set("Response", func(call sobek.ConstructorCall) *sobek.Object {
		obj := call.This
		if len(call.Arguments) > 0 {
			obj.Set("body", call.Argument(0))
		}
		if len(call.Arguments) > 1 {
			obj.Set("options", call.Argument(1))
		}
		return nil
	})

	// Headers constructor
	runtime.Set("Headers", func(call sobek.ConstructorCall) *sobek.Object {
		obj := call.This
		obj.Set("get", func(call sobek.FunctionCall) sobek.Value {
			if len(call.Arguments) > 0 {
				key := call.Argument(0).String()
				return obj.Get(key)
			}
			return sobek.Undefined()
		})
		obj.Set("set", func(call sobek.FunctionCall) sobek.Value {
			if len(call.Arguments) > 1 {
				key := call.Argument(0).String()
				value := call.Argument(1).String()
				obj.Set(key, value)
			}
			return sobek.Undefined()
		})
		return nil
	})

	// FormData constructor
	runtime.Set("FormData", func(call sobek.ConstructorCall) *sobek.Object {
		obj := call.This
		data := make(map[string]string)

		obj.Set("append", func(call sobek.FunctionCall) sobek.Value {
			if len(call.Arguments) > 1 {
				key := call.Argument(0).String()
				value := call.Argument(1).String()
				data[key] = value
			}
			return sobek.Undefined()
		})

		obj.Set("get", func(call sobek.FunctionCall) sobek.Value {
			if len(call.Arguments) > 0 {
				key := call.Argument(0).String()
				if value, exists := data[key]; exists {
					return runtime.ToValue(value)
				}
			}
			return sobek.Undefined()
		})

		return nil
	})

	return nil
}

// Cleanup performs any necessary cleanup
func (f *FetchModule) Cleanup() error {
	// HTTP client doesn't need explicit cleanup
	return nil
}

// IsEnabled checks if the module should be enabled based on configuration
func (f *FetchModule) IsEnabled(enabledModules map[string]bool) bool {
	enabled, exists := enabledModules["fetch"]
	return exists && enabled
}

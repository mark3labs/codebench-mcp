package url

import (
	"net/url"
	"strings"

	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/jsserver/vm"
)

// URLModule provides URL and URLSearchParams
type URLModule struct{}

// NewURLModule creates a new URL module
func NewURLModule() *URLModule {
	return &URLModule{}
}

// Name returns the module name
func (u *URLModule) Name() string {
	return "url"
}

// Setup initializes the URL module in the VM
func (u *URLModule) Setup(runtime *sobek.Runtime, manager *vm.VMManager) error {
	// URL constructor
	runtime.Set("URL", func(call sobek.ConstructorCall) *sobek.Object {
		obj := call.This

		if len(call.Arguments) == 0 {
			panic(runtime.NewTypeError("URL constructor requires a URL string"))
		}

		urlStr := call.Argument(0).String()
		baseURL := ""
		if len(call.Arguments) > 1 {
			baseURL = call.Argument(1).String()
		}

		// Parse the URL
		var parsedURL *url.URL
		var err error

		if baseURL != "" {
			base, err := url.Parse(baseURL)
			if err != nil {
				panic(runtime.NewTypeError("Invalid base URL: " + err.Error()))
			}
			parsedURL, err = base.Parse(urlStr)
		} else {
			parsedURL, err = url.Parse(urlStr)
		}

		if err != nil {
			panic(runtime.NewTypeError("Invalid URL: " + err.Error()))
		}

		// Set properties
		obj.Set("href", parsedURL.String())
		obj.Set("protocol", parsedURL.Scheme+":")
		obj.Set("hostname", parsedURL.Hostname())
		obj.Set("port", parsedURL.Port())
		obj.Set("pathname", parsedURL.Path)
		obj.Set("search", func() string {
			if parsedURL.RawQuery != "" {
				return "?" + parsedURL.RawQuery
			}
			return ""
		}())
		obj.Set("hash", func() string {
			if parsedURL.Fragment != "" {
				return "#" + parsedURL.Fragment
			}
			return ""
		}())
		obj.Set("host", parsedURL.Host)
		obj.Set("origin", parsedURL.Scheme+"://"+parsedURL.Host)

		// searchParams property
		searchParams := u.createURLSearchParams(runtime, parsedURL.Query())
		obj.Set("searchParams", searchParams)

		// toString method
		obj.Set("toString", func(call sobek.FunctionCall) sobek.Value {
			return runtime.ToValue(parsedURL.String())
		})

		return nil
	})

	// URLSearchParams constructor
	runtime.Set("URLSearchParams", func(call sobek.ConstructorCall) *sobek.Object {
		obj := call.This

		params := url.Values{}

		if len(call.Arguments) > 0 {
			arg := call.Argument(0)
			if !sobek.IsUndefined(arg) && !sobek.IsNull(arg) {
				// Parse from string
				if sobek.IsString(arg) {
					queryStr := arg.String()
					if strings.HasPrefix(queryStr, "?") {
						queryStr = queryStr[1:]
					}
					parsed, err := url.ParseQuery(queryStr)
					if err == nil {
						params = parsed
					}
				}
			}
		}

		return u.setupURLSearchParams(runtime, obj, params)
	})

	return nil
}

// createURLSearchParams creates a URLSearchParams object
func (u *URLModule) createURLSearchParams(runtime *sobek.Runtime, params url.Values) sobek.Value {
	obj := runtime.NewObject()
	return u.setupURLSearchParams(runtime, obj, params)
}

// setupURLSearchParams sets up URLSearchParams methods
func (u *URLModule) setupURLSearchParams(runtime *sobek.Runtime, obj *sobek.Object, params url.Values) *sobek.Object {
	// append method
	obj.Set("append", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) >= 2 {
			key := call.Argument(0).String()
			value := call.Argument(1).String()
			params.Add(key, value)
		}
		return sobek.Undefined()
	})

	// delete method
	obj.Set("delete", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) >= 1 {
			key := call.Argument(0).String()
			params.Del(key)
		}
		return sobek.Undefined()
	})

	// get method
	obj.Set("get", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) >= 1 {
			key := call.Argument(0).String()
			value := params.Get(key)
			if value != "" {
				return runtime.ToValue(value)
			}
		}
		return sobek.Null()
	})

	// getAll method
	obj.Set("getAll", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) >= 1 {
			key := call.Argument(0).String()
			values := params[key]
			return runtime.ToValue(values)
		}
		return runtime.ToValue([]string{})
	})

	// has method
	obj.Set("has", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) >= 1 {
			key := call.Argument(0).String()
			return runtime.ToValue(params.Has(key))
		}
		return runtime.ToValue(false)
	})

	// set method
	obj.Set("set", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) >= 2 {
			key := call.Argument(0).String()
			value := call.Argument(1).String()
			params.Set(key, value)
		}
		return sobek.Undefined()
	})

	// toString method
	obj.Set("toString", func(call sobek.FunctionCall) sobek.Value {
		return runtime.ToValue(params.Encode())
	})

	// keys method
	obj.Set("keys", func(call sobek.FunctionCall) sobek.Value {
		var keys []string
		for key := range params {
			keys = append(keys, key)
		}
		return runtime.ToValue(keys)
	})

	// values method
	obj.Set("values", func(call sobek.FunctionCall) sobek.Value {
		var values []string
		for _, vals := range params {
			values = append(values, vals...)
		}
		return runtime.ToValue(values)
	})

	return obj
}

// Cleanup performs any necessary cleanup
func (u *URLModule) Cleanup() error {
	// URL module doesn't need cleanup
	return nil
}

// IsEnabled checks if the module should be enabled based on configuration
func (u *URLModule) IsEnabled(enabledModules map[string]bool) bool {
	enabled, exists := enabledModules["url"]
	return exists && enabled
}

package encoding

import (
	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/jsserver/vm"
)

// EncodingModule provides TextEncoder and TextDecoder
type EncodingModule struct{}

// NewEncodingModule creates a new encoding module
func NewEncodingModule() *EncodingModule {
	return &EncodingModule{}
}

// Name returns the module name
func (e *EncodingModule) Name() string {
	return "encoding"
}

// Setup initializes the encoding module in the VM
func (e *EncodingModule) Setup(runtime *sobek.Runtime, manager *vm.VMManager) error {
	// TextEncoder constructor
	runtime.Set("TextEncoder", func(call sobek.ConstructorCall) *sobek.Object {
		obj := call.This

		// encode method
		obj.Set("encode", func(call sobek.FunctionCall) sobek.Value {
			if len(call.Arguments) == 0 {
				return runtime.ToValue([]byte{})
			}
			text := call.Argument(0).String()
			return runtime.ToValue([]byte(text))
		})

		// encoding property
		obj.Set("encoding", "utf-8")

		return nil
	})

	// TextDecoder constructor
	runtime.Set("TextDecoder", func(call sobek.ConstructorCall) *sobek.Object {
		obj := call.This

		encoding := "utf-8"
		if len(call.Arguments) > 0 {
			encoding = call.Argument(0).String()
		}

		// decode method
		obj.Set("decode", func(call sobek.FunctionCall) sobek.Value {
			if len(call.Arguments) == 0 {
				return runtime.ToValue("")
			}

			arg := call.Argument(0)
			var bytes []byte

			// Handle different input types
			if exported := arg.Export(); exported != nil {
				switch v := exported.(type) {
				case []byte:
					bytes = v
				case []any:
					// Convert array of numbers to bytes
					bytes = make([]byte, len(v))
					for i, val := range v {
						if num, ok := val.(float64); ok {
							bytes[i] = byte(int(num))
						}
					}
				default:
					// Convert to string and then bytes
					bytes = []byte(arg.String())
				}
			} else {
				bytes = []byte(arg.String())
			}

			return runtime.ToValue(string(bytes))
		})

		// encoding property
		obj.Set("encoding", encoding)

		return nil
	})

	return nil
}

// Cleanup performs any necessary cleanup
func (e *EncodingModule) Cleanup() error {
	// Encoding module doesn't need cleanup
	return nil
}

// IsEnabled checks if the module should be enabled based on configuration
func (e *EncodingModule) IsEnabled(enabledModules map[string]bool) bool {
	enabled, exists := enabledModules["encoding"]
	return exists && enabled
}

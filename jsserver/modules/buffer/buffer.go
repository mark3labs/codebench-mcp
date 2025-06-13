package buffer

import (
	"encoding/base64"
	"encoding/hex"

	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/jsserver/vm"
)

// BufferModule provides Buffer global for binary data handling
type BufferModule struct{}

// NewBufferModule creates a new buffer module
func NewBufferModule() *BufferModule {
	return &BufferModule{}
}

// Name returns the module name
func (b *BufferModule) Name() string {
	return "buffer"
}

// Setup initializes the buffer module in the VM
func (b *BufferModule) Setup(runtime *sobek.Runtime, manager *vm.VMManager) error {
	// Buffer constructor
	runtime.Set("Buffer", func(call sobek.ConstructorCall) *sobek.Object {
		obj := call.This
		var data []byte

		if len(call.Arguments) > 0 {
			arg := call.Argument(0)

			// Handle different input types
			if sobek.IsString(arg) {
				encoding := "utf8"
				if len(call.Arguments) > 1 {
					encoding = call.Argument(1).String()
				}

				str := arg.String()
				switch encoding {
				case "base64":
					decoded, err := base64.StdEncoding.DecodeString(str)
					if err != nil {
						panic(runtime.NewGoError(err))
					}
					data = decoded
				case "hex":
					decoded, err := hex.DecodeString(str)
					if err != nil {
						panic(runtime.NewGoError(err))
					}
					data = decoded
				default: // utf8
					data = []byte(str)
				}
			} else if sobek.IsNumber(arg) {
				// Create buffer of specified size
				size := arg.ToInteger()
				data = make([]byte, size)
			} else {
				// Try to convert to array
				exported := arg.Export()
				if arr, ok := exported.([]interface{}); ok {
					data = make([]byte, len(arr))
					for i, v := range arr {
						if num, ok := v.(float64); ok {
							data[i] = byte(int(num))
						}
					}
				}
			}
		}

		// Store the data
		obj.Set("__data__", data)
		obj.Set("length", len(data))

		// toString method
		obj.Set("toString", func(call sobek.FunctionCall) sobek.Value {
			encoding := "utf8"
			if len(call.Arguments) > 0 {
				encoding = call.Argument(0).String()
			}

			dataVal := obj.Get("__data__")
			data := dataVal.Export().([]byte)
			switch encoding {
			case "base64":
				return runtime.ToValue(base64.StdEncoding.EncodeToString(data))
			case "hex":
				return runtime.ToValue(hex.EncodeToString(data))
			default: // utf8
				return runtime.ToValue(string(data))
			}
		})

		// slice method
		obj.Set("slice", func(call sobek.FunctionCall) sobek.Value {
			dataVal := obj.Get("__data__")
			data := dataVal.Export().([]byte)
			start := 0
			end := len(data)

			if len(call.Arguments) > 0 {
				start = int(call.Argument(0).ToInteger())
				if start < 0 {
					start = len(data) + start
				}
			}
			if len(call.Arguments) > 1 {
				end = int(call.Argument(1).ToInteger())
				if end < 0 {
					end = len(data) + end
				}
			}

			if start < 0 {
				start = 0
			}
			if end > len(data) {
				end = len(data)
			}
			if start > end {
				start = end
			}

			sliced := data[start:end]

			// Create new Buffer object
			newBuffer := runtime.NewObject()
			newBuffer.Set("__data__", sliced)
			newBuffer.Set("length", len(sliced))

			// Copy methods to new buffer
			newBuffer.Set("toString", obj.Get("toString"))
			newBuffer.Set("slice", obj.Get("slice"))

			return newBuffer
		})

		return nil
	})

	// Buffer.from static method
	bufferObj := runtime.Get("Buffer").ToObject(runtime)
	bufferObj.Set("from", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			return runtime.NewObject()
		}

		// Create new Buffer using constructor logic
		constructor, _ := sobek.AssertFunction(runtime.Get("Buffer"))
		result, err := constructor(sobek.Undefined(), call.Arguments...)
		if err != nil {
			panic(runtime.NewGoError(err))
		}
		return result
	})

	// Buffer.alloc static method
	bufferObj.Set("alloc", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			return runtime.NewObject()
		}

		size := call.Argument(0).ToInteger()
		fill := byte(0)
		if len(call.Arguments) > 1 {
			fill = byte(call.Argument(1).ToInteger())
		}

		data := make([]byte, size)
		for i := range data {
			data[i] = fill
		}

		newBuffer := runtime.NewObject()
		newBuffer.Set("__data__", data)
		newBuffer.Set("length", len(data))

		// Add methods
		newBuffer.Set("toString", bufferObj.Get("toString"))
		newBuffer.Set("slice", bufferObj.Get("slice"))

		return newBuffer
	})

	return nil
}

// Cleanup performs any necessary cleanup
func (b *BufferModule) Cleanup() error {
	// Buffer module doesn't need cleanup
	return nil
}

// IsEnabled checks if the module should be enabled based on configuration
func (b *BufferModule) IsEnabled(enabledModules map[string]bool) bool {
	enabled, exists := enabledModules["buffer"]
	return exists && enabled
}

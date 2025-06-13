package kv

import (
	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/jsserver/vm"
)

// KVModule provides key-value storage per VM instance
type KVModule struct {
	store map[string]interface{} // Per-VM instance storage
}

// NewKVModule creates a new KV module with isolated storage
func NewKVModule() *KVModule {
	return &KVModule{
		store: make(map[string]interface{}),
	}
}

// Name returns the module name
func (kv *KVModule) Name() string {
	return "kv"
}

// Setup initializes the KV module in the VM
func (kv *KVModule) Setup(runtime *sobek.Runtime, manager *vm.VMManager) error {
	kvObj := runtime.NewObject()

	// kv.get(key) - retrieve a value
	kvObj.Set("get", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			return sobek.Undefined()
		}
		key := call.Argument(0).String()
		value, exists := kv.store[key]
		if !exists {
			return sobek.Undefined()
		}
		return runtime.ToValue(value)
	})

	// kv.set(key, value) - store a value
	kvObj.Set("set", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 2 {
			return runtime.ToValue(false)
		}
		key := call.Argument(0).String()
		value := call.Argument(1).Export()
		kv.store[key] = value
		return runtime.ToValue(true)
	})

	// kv.delete(key) - remove a value
	kvObj.Set("delete", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			return runtime.ToValue(false)
		}
		key := call.Argument(0).String()
		_, exists := kv.store[key]
		if exists {
			delete(kv.store, key)
			return runtime.ToValue(true)
		}
		return runtime.ToValue(false)
	})

	// kv.list() - list all keys
	kvObj.Set("list", func(call sobek.FunctionCall) sobek.Value {
		keys := make([]string, 0, len(kv.store))
		for key := range kv.store {
			keys = append(keys, key)
		}
		return runtime.ToValue(keys)
	})

	// kv.clear() - clear all data
	kvObj.Set("clear", func(call sobek.FunctionCall) sobek.Value {
		kv.store = make(map[string]interface{})
		return runtime.ToValue(true)
	})

	// kv.has(key) - check if key exists
	kvObj.Set("has", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			return runtime.ToValue(false)
		}
		key := call.Argument(0).String()
		_, exists := kv.store[key]
		return runtime.ToValue(exists)
	})

	// kv.size() - get number of stored items
	kvObj.Set("size", func(call sobek.FunctionCall) sobek.Value {
		return runtime.ToValue(len(kv.store))
	})

	runtime.Set("kv", kvObj)
	return nil
}

// Cleanup performs any necessary cleanup
func (kv *KVModule) Cleanup() error {
	// Clear the store on cleanup
	kv.store = nil
	return nil
}

// IsEnabled checks if the module should be enabled based on configuration
func (kv *KVModule) IsEnabled(enabledModules map[string]bool) bool {
	enabled, exists := enabledModules["kv"]
	return exists && enabled
}

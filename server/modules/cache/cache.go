package cache

import (
	"context"
	"sync"
	"time"

	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/server/vm"
)

// CacheModule provides in-memory caching with TTL support
type CacheModule struct {
	cache Cache
}

// NewCacheModule creates a new cache module
func NewCacheModule() *CacheModule {
	return &CacheModule{
		cache: NewCache(),
	}
}

// Name returns the module name
func (c *CacheModule) Name() string {
	return "cache"
}

// Setup initializes the cache module in the VM
func (c *CacheModule) Setup(runtime *sobek.Runtime, manager *vm.VMManager) error {
	// No setup needed - the module will be available via require()
	return nil
}

// CreateModuleObject creates the cache object when required
func (c *CacheModule) CreateModuleObject(runtime *sobek.Runtime) sobek.Value {
	return c.createCacheObject(runtime)
}

// createCacheObject creates the cache object with all methods
func (c *CacheModule) createCacheObject(runtime *sobek.Runtime) sobek.Value {
	cache := runtime.NewObject()

	// get(key) - returns string value or undefined
	cache.Set("get", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			return sobek.Undefined()
		}
		
		key := call.Argument(0).String()
		if bytes, err := c.cache.Get(context.Background(), key); err == nil && bytes != nil {
			return runtime.ToValue(string(bytes))
		}
		return sobek.Undefined()
	})

	// getBytes(key) - returns ArrayBuffer or undefined
	cache.Set("getBytes", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			return sobek.Undefined()
		}
		
		key := call.Argument(0).String()
		if bytes, err := c.cache.Get(context.Background(), key); err == nil && bytes != nil {
			return runtime.ToValue(runtime.NewArrayBuffer(bytes))
		}
		return sobek.Undefined()
	})

	// set(key, value, ttlMs?) - stores string value with optional TTL in milliseconds
	cache.Set("set", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 2 {
			panic(runtime.NewTypeError("cache.set requires at least 2 arguments"))
		}
		
		key := call.Argument(0).String()
		value := []byte(call.Argument(1).String())
		
		var timeout time.Duration
		if len(call.Arguments) > 2 && !sobek.IsUndefined(call.Argument(2)) {
			timeout = time.Millisecond * time.Duration(call.Argument(2).ToInteger())
		}
		
		err := c.cache.Set(context.Background(), key, value, timeout)
		if err != nil {
			panic(runtime.NewGoError(err))
		}
		
		return sobek.Undefined()
	})

	// setBytes(key, arrayBuffer, ttlMs?) - stores ArrayBuffer with optional TTL
	cache.Set("setBytes", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 2 {
			panic(runtime.NewTypeError("cache.setBytes requires at least 2 arguments"))
		}
		
		key := call.Argument(0).String()
		
		// Convert value to bytes
		var value []byte
		arg := call.Argument(1)
		if exported := arg.Export(); exported != nil {
			switch v := exported.(type) {
			case []byte:
				value = v
			case []any:
				// Convert array of numbers to bytes
				value = make([]byte, len(v))
				for i, val := range v {
					if num, ok := val.(float64); ok {
						value[i] = byte(int(num))
					}
				}
			default:
				// Convert to string and then bytes
				value = []byte(arg.String())
			}
		} else {
			value = []byte(arg.String())
		}
		
		var timeout time.Duration
		if len(call.Arguments) > 2 && !sobek.IsUndefined(call.Argument(2)) {
			timeout = time.Millisecond * time.Duration(call.Argument(2).ToInteger())
		}
		
		err := c.cache.Set(context.Background(), key, value, timeout)
		if err != nil {
			panic(runtime.NewGoError(err))
		}
		
		return sobek.Undefined()
	})

	// del(key) - removes key from cache
	cache.Set("del", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			return sobek.Undefined()
		}
		
		key := call.Argument(0).String()
		err := c.cache.Del(context.Background(), key)
		if err != nil {
			panic(runtime.NewGoError(err))
		}
		
		return sobek.Undefined()
	})

	return cache
}

// Cleanup performs any necessary cleanup
func (c *CacheModule) Cleanup() error {
	// Memory cache doesn't need explicit cleanup
	return nil
}

// IsEnabled checks if the module should be enabled based on configuration
func (c *CacheModule) IsEnabled(enabledModules map[string]bool) bool {
	enabled, exists := enabledModules["cache"]
	return exists && enabled
}

// Cache interface for storing bytes with TTL
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, timeout time.Duration) error
	Del(ctx context.Context, key string) error
}

// memoryCache is an implementation of Cache that stores bytes in in-memory
type memoryCache struct {
	sync.Mutex
	items   map[string][]byte
	timeout map[string]int64
}

// Get returns the []byte if existing and not expired
func (c *memoryCache) Get(_ context.Context, key string) ([]byte, error) {
	c.Lock()
	defer c.Unlock()
	
	if ddl, exist := c.timeout[key]; exist {
		if time.Now().UnixMilli() > ddl {
			delete(c.items, key)
			delete(c.timeout, key)
			return nil, nil
		}
	}
	
	return c.items[key], nil
}

// Set saves []byte to the cache with key and optional timeout
func (c *memoryCache) Set(_ context.Context, key string, value []byte, timeout time.Duration) error {
	c.Lock()
	defer c.Unlock()
	
	c.items[key] = value
	if timeout > 0 {
		c.timeout[key] = time.Now().Add(timeout).UnixMilli()
	} else {
		// No timeout - store indefinitely
		delete(c.timeout, key)
	}
	
	return nil
}

// Del removes key from the cache
func (c *memoryCache) Del(_ context.Context, key string) error {
	c.Lock()
	defer c.Unlock()
	
	delete(c.items, key)
	delete(c.timeout, key)
	
	return nil
}

// NewCache returns a new Cache that will store items in in-memory
func NewCache() Cache {
	return &memoryCache{
		items:   make(map[string][]byte),
		timeout: make(map[string]int64),
	}
}
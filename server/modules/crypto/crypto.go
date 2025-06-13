package crypto

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"hash"

	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/server/vm"
)

// CryptoModule provides cryptographic functions
type CryptoModule struct{}

// NewCryptoModule creates a new crypto module
func NewCryptoModule() *CryptoModule {
	return &CryptoModule{}
}

// Name returns the module name
func (c *CryptoModule) Name() string {
	return "crypto"
}

// Encoder represents encoded data that can be output in different formats
type Encoder struct {
	data []byte
}

// hex returns the hex encoding of the data
func (e *Encoder) hex() string {
	return hex.EncodeToString(e.data)
}

// base64 returns the base64 encoding of the data
func (e *Encoder) base64() string {
	return base64.StdEncoding.EncodeToString(e.data)
}

// bytes returns the raw bytes
func (e *Encoder) bytes() []byte {
	return e.data
}

// Setup initializes the crypto module in the VM
func (c *CryptoModule) Setup(runtime *sobek.Runtime, manager *vm.VMManager) error {
	// No setup needed - the module will be available via require()
	return nil
}

// CreateModuleObject creates the crypto object when required
func (c *CryptoModule) CreateModuleObject(runtime *sobek.Runtime) sobek.Value {
	return c.createCryptoObject(runtime)
}

// createCryptoObject creates the crypto module object
func (c *CryptoModule) createCryptoObject(runtime *sobek.Runtime) sobek.Value {
	crypto := runtime.NewObject()

	// Hash functions
	crypto.Set("md5", func(call sobek.FunctionCall) sobek.Value {
		return c.hash(runtime, "md5", call.Arguments)
	})

	crypto.Set("sha1", func(call sobek.FunctionCall) sobek.Value {
		return c.hash(runtime, "sha1", call.Arguments)
	})

	crypto.Set("sha256", func(call sobek.FunctionCall) sobek.Value {
		return c.hash(runtime, "sha256", call.Arguments)
	})

	crypto.Set("sha384", func(call sobek.FunctionCall) sobek.Value {
		return c.hash(runtime, "sha384", call.Arguments)
	})

	crypto.Set("sha512", func(call sobek.FunctionCall) sobek.Value {
		return c.hash(runtime, "sha512", call.Arguments)
	})

	// HMAC functions
	crypto.Set("hmac", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 3 {
			panic(runtime.NewTypeError("hmac requires algorithm, key, and data"))
		}
		algorithm := call.Argument(0).String()
		key := call.Argument(1)
		data := call.Argument(2)
		return c.hmac(runtime, algorithm, key, data)
	})

	// Random bytes
	crypto.Set("randomBytes", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			panic(runtime.NewTypeError("randomBytes requires size argument"))
		}
		size := int(call.Argument(0).ToInteger())
		if size < 1 {
			panic(runtime.NewTypeError("invalid size"))
		}
		bytes := make([]byte, size)
		if _, err := rand.Read(bytes); err != nil {
			panic(runtime.NewGoError(err))
		}
		return runtime.ToValue(bytes)
	})

	return crypto
}

// hash performs hashing with the specified algorithm
func (c *CryptoModule) hash(runtime *sobek.Runtime, algorithm string, args []sobek.Value) sobek.Value {
	if len(args) == 0 {
		panic(runtime.NewTypeError("hash function requires data argument"))
	}

	data := c.toBytes(args[0])
	hasher := c.getHasher(algorithm)
	if hasher == nil {
		panic(runtime.NewTypeError("unsupported hash algorithm: " + algorithm))
	}

	hasher.Write(data)
	result := hasher.Sum(nil)

	encoder := &Encoder{data: result}

	// Create encoder object with methods
	encoderObj := runtime.NewObject()
	encoderObj.Set("hex", func(call sobek.FunctionCall) sobek.Value {
		return runtime.ToValue(encoder.hex())
	})
	encoderObj.Set("base64", func(call sobek.FunctionCall) sobek.Value {
		return runtime.ToValue(encoder.base64())
	})
	encoderObj.Set("bytes", func(call sobek.FunctionCall) sobek.Value {
		return runtime.ToValue(encoder.bytes())
	})

	return encoderObj
}

// hmac performs HMAC with the specified algorithm
func (c *CryptoModule) hmac(runtime *sobek.Runtime, algorithm string, key, data sobek.Value) sobek.Value {
	keyBytes := c.toBytes(key)
	dataBytes := c.toBytes(data)

	hasher := c.getHasher(algorithm)
	if hasher == nil {
		panic(runtime.NewTypeError("unsupported hash algorithm: " + algorithm))
	}

	h := hmac.New(func() hash.Hash { return c.getHasher(algorithm) }, keyBytes)
	h.Write(dataBytes)
	result := h.Sum(nil)

	encoder := &Encoder{data: result}

	// Create encoder object with methods
	encoderObj := runtime.NewObject()
	encoderObj.Set("hex", func(call sobek.FunctionCall) sobek.Value {
		return runtime.ToValue(encoder.hex())
	})
	encoderObj.Set("base64", func(call sobek.FunctionCall) sobek.Value {
		return runtime.ToValue(encoder.base64())
	})
	encoderObj.Set("bytes", func(call sobek.FunctionCall) sobek.Value {
		return runtime.ToValue(encoder.bytes())
	})

	return encoderObj
}

// getHasher returns a hash function for the given algorithm
func (c *CryptoModule) getHasher(algorithm string) hash.Hash {
	switch algorithm {
	case "md5":
		return md5.New()
	case "sha1":
		return sha1.New()
	case "sha256":
		return sha256.New()
	case "sha384":
		return sha512.New384()
	case "sha512":
		return sha512.New()
	default:
		return nil
	}
}

// toBytes converts a Sobek value to bytes
func (c *CryptoModule) toBytes(value sobek.Value) []byte {
	if value == nil || sobek.IsUndefined(value) || sobek.IsNull(value) {
		return []byte{}
	}

	// Try to get as bytes first
	if exported := value.Export(); exported != nil {
		if bytes, ok := exported.([]byte); ok {
			return bytes
		}
	}

	// Convert to string and then bytes
	return []byte(value.String())
}

// Cleanup performs any necessary cleanup
func (c *CryptoModule) Cleanup() error {
	// Crypto module doesn't need cleanup
	return nil
}

// IsEnabled checks if the module should be enabled based on configuration
func (c *CryptoModule) IsEnabled(enabledModules map[string]bool) bool {
	enabled, exists := enabledModules["crypto"]
	return exists && enabled
}

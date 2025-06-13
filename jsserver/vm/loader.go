package vm

import (
	"fmt"
	"sync"

	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/internal/logger"
)

// ModuleLoader provides a global require system for modules
// Based on ski's loader pattern but simplified for our use case
type ModuleLoader struct {
	modules sync.Map // map[string]Module
	aliases sync.Map // map[string]string - maps alias to module name
}

// NewModuleLoader creates a new module loader
func NewModuleLoader() *ModuleLoader {
	return &ModuleLoader{}
}

// RegisterModule registers a module with the loader
func (l *ModuleLoader) RegisterModule(module Module) {
	l.modules.Store(module.Name(), module)
	logger.Debug("Module registered with loader", "name", module.Name())
	
	// Register common aliases
	switch module.Name() {
	case "http":
		l.aliases.Store("http/server", "http")
		logger.Debug("Module alias registered", "alias", "http/server", "module", "http")
	case "crypto":
		l.aliases.Store("crypto", "crypto")
		logger.Debug("Module alias registered", "alias", "crypto", "module", "crypto")
	case "cache":
		l.aliases.Store("cache", "cache")
		logger.Debug("Module alias registered", "alias", "cache", "module", "cache")
	}
}

// EnableRequire sets up the global require function in the runtime
func (l *ModuleLoader) EnableRequire(rt *sobek.Runtime, enabledModules map[string]bool) {
	rt.Set("require", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			panic(rt.NewTypeError("require() expects a module name"))
		}

		moduleName := call.Argument(0).String()
		logger.Debug("Require called", "module", moduleName)

		// Check for aliases first
		if aliasTarget, ok := l.aliases.Load(moduleName); ok {
			moduleName = aliasTarget.(string)
			logger.Debug("Module alias resolved", "alias", call.Argument(0).String(), "target", moduleName)
		}

		// Look up the module
		if moduleInterface, ok := l.modules.Load(moduleName); ok {
			module := moduleInterface.(Module)
			logger.Debug("Module found", "name", moduleName)
			
			// Check if module is enabled
			if !module.IsEnabled(enabledModules) {
				logger.Debug("Module not enabled", "name", moduleName)
				panic(rt.NewTypeError(fmt.Sprintf("Module '%s' is not enabled", moduleName)))
			}
			
			// Create the module object
			if moduleCreator, ok := module.(ModuleCreator); ok {
				return moduleCreator.CreateModuleObject(rt)
			}
			
			// Fallback: return undefined for modules that don't implement ModuleCreator
			logger.Debug("Module doesn't implement ModuleCreator", "name", moduleName)
			return sobek.Undefined()
		}

		// Module not found
		logger.Debug("Module not found", "name", moduleName)
		panic(rt.NewTypeError(fmt.Sprintf("Cannot find module '%s'", moduleName)))
	})
	logger.Debug("Global require function enabled")
}

// ModuleCreator interface for modules that can create their own objects
// This replaces the old require override pattern
type ModuleCreator interface {
	CreateModuleObject(runtime *sobek.Runtime) sobek.Value
}

// GlobalModule interface for modules that provide global objects
// These modules will be automatically available as globals (like fetch, console)
type GlobalModule interface {
	GetGlobalName() string
	CreateGlobalObject(runtime *sobek.Runtime) sobek.Value
}

// SetupGlobals sets up global objects for modules that implement GlobalModule
func (l *ModuleLoader) SetupGlobals(rt *sobek.Runtime, enabledModules map[string]bool) {
	l.modules.Range(func(key, value any) bool {
		module := value.(Module)
		if globalModule, ok := module.(GlobalModule); ok {
			// Check if module is enabled
			if module.IsEnabled(enabledModules) {
				globalName := globalModule.GetGlobalName()
				globalObject := globalModule.CreateGlobalObject(rt)
				rt.Set(globalName, globalObject)
				logger.Debug("Global object set", "name", globalName)
			} else {
				logger.Debug("Global module not enabled", "name", module.Name())
			}
		}
		return true
	})
}
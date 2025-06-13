package vm

import (
	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/internal/logger"
)

// Module interface defines how modules integrate with the VM
type Module interface {
	Name() string
	Setup(runtime *sobek.Runtime, manager *VMManager) error
	Cleanup() error
	IsEnabled(enabledModules map[string]bool) bool
}

// ModuleRegistry manages available modules
type ModuleRegistry struct {
	modules map[string]Module
}

// NewModuleRegistry creates a new module registry
func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		modules: make(map[string]Module),
	}
}

// Register adds a module to the registry
func (r *ModuleRegistry) Register(module Module) {
	r.modules[module.Name()] = module
}

// Get retrieves a module by name
func (r *ModuleRegistry) Get(name string) (Module, bool) {
	module, exists := r.modules[name]
	return module, exists
}

// GetEnabled returns all enabled modules based on configuration
func (r *ModuleRegistry) GetEnabled(enabledModules map[string]bool) []Module {
	logger.Debug("Getting enabled modules", "enabledMap", enabledModules)
	var enabled []Module
	for _, module := range r.modules {
		logger.Debug("Checking module", "name", module.Name(), "enabled", module.IsEnabled(enabledModules))
		if module.IsEnabled(enabledModules) {
			enabled = append(enabled, module)
		}
	}
	logger.Debug("Enabled modules found", "count", len(enabled))
	return enabled
}

// List returns all registered module names
func (r *ModuleRegistry) List() []string {
	var names []string
	for name := range r.modules {
		names = append(names, name)
	}
	return names
}

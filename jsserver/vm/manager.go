package vm

import (
	"context"

	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/internal/logger"
)

// VMManager manages Sobek VM instances
type VMManager struct {
	enabledModules map[string]bool
	registry       *ModuleRegistry
	loader         *ModuleLoader
}

// NewVMManager creates a new VM manager with specified enabled modules
func NewVMManager(enabledModules []string) *VMManager {
	enabledMap := make(map[string]bool)
	for _, module := range enabledModules {
		enabledMap[module] = true
	}

	return &VMManager{
		enabledModules: enabledMap,
		registry:       NewModuleRegistry(),
		loader:         NewModuleLoader(),
	}
}

// RegisterModule adds a module to the manager
func (m *VMManager) RegisterModule(module Module) error {
	m.registry.Register(module)
	m.loader.RegisterModule(module)
	return nil
}

// CreateVM creates a new VM instance with all enabled modules
// Each VM is completely isolated
func (m *VMManager) CreateVM(ctx context.Context) (*VM, error) {
	logger.Debug("Creating new VM instance")
	
	// Create new Sobek runtime
	rt := sobek.New()

	// Create event loop
	eventLoop := NewEventLoop()

	vm := &VM{
		runtime:   rt,
		manager:   m,
		ctx:       ctx,
		eventLoop: eventLoop,
	}

	// Store VM reference in runtime for event loop access
	_ = rt.GlobalObject().SetSymbol(symbolVM, &vmSelf{vm: vm})
	logger.Debug("VM symbol stored in runtime")

	// Setup global require function
	m.loader.EnableRequire(rt, m.enabledModules)
	logger.Debug("Global require function enabled")

	// Setup all enabled modules
	enabledModules := m.registry.GetEnabled(m.enabledModules)
	logger.Debug("Setting up enabled modules", "count", len(enabledModules))
	for _, module := range enabledModules {
		logger.Debug("Setting up module", "name", module.Name())
		if err := module.Setup(rt, m); err != nil {
			logger.Debug("Module setup failed", "name", module.Name(), "error", err)
			return nil, err
		}
		logger.Debug("Module setup completed", "name", module.Name())
	}

	// Setup global objects for modules that provide them
	m.loader.SetupGlobals(rt, m.enabledModules)
	logger.Debug("Global objects setup completed")

	logger.Debug("VM creation completed")
	return vm, nil
}

// GetEnabledModules returns the list of enabled module names
func (m *VMManager) GetEnabledModules() []string {
	var enabled []string
	for module := range m.enabledModules {
		enabled = append(enabled, module)
	}
	logger.Debug("Enabled modules", "modules", enabled)
	return enabled
}

// VM wraps a Sobek runtime with event loop support
type VM struct {
	runtime   *sobek.Runtime
	manager   *VMManager
	ctx       context.Context
	eventLoop *EventLoop
}

// RunString executes JavaScript code in the VM with event loop support
// This matches ski's pattern where RunString always uses the event loop
func (vm *VM) RunString(code string) (ret sobek.Value, err error) {
	err = vm.runWithEventLoop(func() error {
		ret, err = vm.runtime.RunString(code)
		return err
	})
	return
}

// runWithEventLoop executes a task in the event loop (similar to ski's Run method)
func (vm *VM) runWithEventLoop(task func() error) error {
	// Clear any previous interrupt
	vm.runtime.ClearInterrupt()
	
	// Set up context cancellation to interrupt the runtime if needed
	if vm.ctx != nil {
		go func() {
			<-vm.ctx.Done()
			vm.runtime.Interrupt(vm.ctx.Err())
			vm.eventLoop.Stop(vm.ctx.Err())
		}()
	}
	
	return vm.eventLoop.Start(task)
}

// SetGlobal sets a global variable in the VM
func (vm *VM) SetGlobal(name string, value interface{}) {
	vm.runtime.Set(name, value)
}

// Runtime returns the underlying Sobek runtime
func (vm *VM) Runtime() *sobek.Runtime {
	return vm.runtime
}

// Close cleans up the VM and its modules
func (vm *VM) Close() error {
	// Cleanup all modules
	enabledModules := vm.manager.registry.GetEnabled(vm.manager.enabledModules)
	for _, module := range enabledModules {
		if err := module.Cleanup(); err != nil {
			// Log error but continue cleanup
			continue
		}
	}

	return nil
}

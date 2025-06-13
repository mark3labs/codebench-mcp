package vm

import (
	"context"

	"github.com/grafana/sobek"
)

// VMManager manages Sobek VM instances
type VMManager struct {
	enabledModules map[string]bool
	registry       *ModuleRegistry
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
	}
}

// RegisterModule adds a module to the manager
func (m *VMManager) RegisterModule(module Module) error {
	m.registry.Register(module)
	return nil
}

// CreateVM creates a new VM instance with all enabled modules
// Each VM is completely isolated
func (m *VMManager) CreateVM(ctx context.Context) (*VM, error) {
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

	// Setup all enabled modules
	enabledModules := m.registry.GetEnabled(m.enabledModules)
	for _, module := range enabledModules {
		if err := module.Setup(rt, m); err != nil {
			return nil, err
		}
	}

	return vm, nil
}

// GetEnabledModules returns the list of enabled module names
func (m *VMManager) GetEnabledModules() []string {
	var enabled []string
	for module := range m.enabledModules {
		enabled = append(enabled, module)
	}
	return enabled
}

// VM wraps a Sobek runtime with event loop support
type VM struct {
	runtime   *sobek.Runtime
	manager   *VMManager
	ctx       context.Context
	eventLoop *EventLoop
}

// RunString executes JavaScript code in the VM
func (vm *VM) RunString(code string) (sobek.Value, error) {
	return vm.runtime.RunString(code)
}

// RunStringWithEventLoop executes JavaScript code with event loop support
// This matches ski's pattern where RunString calls Run() which uses the event loop
func (vm *VM) RunStringWithEventLoop(code string) (ret sobek.Value, err error) {
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

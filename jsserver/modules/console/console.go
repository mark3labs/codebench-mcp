package console

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/grafana/sobek"
	"github.com/mark3labs/codebench-mcp/jsserver/vm"
)

// ConsoleModule provides console.log, console.error, etc.
type ConsoleModule struct {
	logger *slog.Logger
}

// NewConsoleModule creates a new console module
func NewConsoleModule(logger *slog.Logger) *ConsoleModule {
	if logger == nil {
		logger = slog.Default()
	}
	return &ConsoleModule{
		logger: logger,
	}
}

// Name returns the module name
func (c *ConsoleModule) Name() string {
	return "console"
}

// formatArgs formats console arguments for output
func (c *ConsoleModule) formatArgs(args []sobek.Value) string {
	var parts []string
	for _, arg := range args {
		exported := arg.Export()
		parts = append(parts, fmt.Sprintf("%v", exported))
	}
	return strings.Join(parts, " ")
}

// Setup initializes the console module in the VM
func (c *ConsoleModule) Setup(runtime *sobek.Runtime, manager *vm.VMManager) error {
	console := runtime.NewObject()

	// console.log
	console.Set("log", func(call sobek.FunctionCall) sobek.Value {
		message := c.formatArgs(call.Arguments)
		c.logger.Info(message)
		return sobek.Undefined()
	})

	// console.error
	console.Set("error", func(call sobek.FunctionCall) sobek.Value {
		message := c.formatArgs(call.Arguments)
		c.logger.Error(message)
		return sobek.Undefined()
	})

	// console.warn
	console.Set("warn", func(call sobek.FunctionCall) sobek.Value {
		message := c.formatArgs(call.Arguments)
		c.logger.Warn(message)
		return sobek.Undefined()
	})

	// console.info
	console.Set("info", func(call sobek.FunctionCall) sobek.Value {
		message := c.formatArgs(call.Arguments)
		c.logger.Info(message)
		return sobek.Undefined()
	})

	// console.debug
	console.Set("debug", func(call sobek.FunctionCall) sobek.Value {
		message := c.formatArgs(call.Arguments)
		c.logger.Debug(message)
		return sobek.Undefined()
	})

	runtime.Set("console", console)
	return nil
}

// Cleanup performs any necessary cleanup
func (c *ConsoleModule) Cleanup() error {
	// Console module doesn't need cleanup
	return nil
}

// IsEnabled checks if the module should be enabled based on configuration
func (c *ConsoleModule) IsEnabled(enabledModules map[string]bool) bool {
	// Console is always enabled as it's essential for debugging
	return true
}

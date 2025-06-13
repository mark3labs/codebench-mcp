package console

import (
	"fmt"
	"strings"

	"github.com/grafana/sobek"
)

// ConsoleModule provides console.log, console.error, etc.
type ConsoleModule struct {
	output *strings.Builder
}

// NewConsoleModule creates a new console module
func NewConsoleModule(output *strings.Builder) *ConsoleModule {
	if output == nil {
		output = &strings.Builder{}
	}
	return &ConsoleModule{
		output: output,
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

// writeMessage writes a message to the output
func (c *ConsoleModule) writeMessage(message string) {
	if c.output != nil {
		c.output.WriteString(message)
		c.output.WriteString("\n")
	}
}

// GetOutput returns the captured console output
func (c *ConsoleModule) GetOutput() string {
	if c.output == nil {
		return ""
	}
	return c.output.String()
}

// Setup initializes the console module in the VM
func (c *ConsoleModule) Setup(runtime *sobek.Runtime) error {
	console := runtime.NewObject()

	// console.log
	console.Set("log", func(call sobek.FunctionCall) sobek.Value {
		message := c.formatArgs(call.Arguments)
		c.writeMessage(message)
		return sobek.Undefined()
	})

	// console.error
	console.Set("error", func(call sobek.FunctionCall) sobek.Value {
		message := c.formatArgs(call.Arguments)
		c.writeMessage(message)
		return sobek.Undefined()
	})

	// console.warn
	console.Set("warn", func(call sobek.FunctionCall) sobek.Value {
		message := c.formatArgs(call.Arguments)
		c.writeMessage(message)
		return sobek.Undefined()
	})

	// console.info
	console.Set("info", func(call sobek.FunctionCall) sobek.Value {
		message := c.formatArgs(call.Arguments)
		c.writeMessage(message)
		return sobek.Undefined()
	})

	// console.debug
	console.Set("debug", func(call sobek.FunctionCall) sobek.Value {
		message := c.formatArgs(call.Arguments)
		c.writeMessage(message)
		return sobek.Undefined()
	})

	// Set console as global
	runtime.Set("console", console)
	return nil
}
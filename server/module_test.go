package server

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleConfiguration_EnabledModules(t *testing.T) {
	// Test with only fetch enabled
	config := ModuleConfig{
		EnabledModules: []string{"fetch"},
	}
	handler := NewJSHandlerWithConfig(config)

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			// Test fetch availability
			const fetchAvailable = typeof fetch !== 'undefined';
			const httpAvailable = typeof require !== 'undefined';
			
			console.log("fetch available:", fetchAvailable);
			console.log("http available:", httpAvailable);
			
			"module test completed";
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text

	// Should have fetch available
	assert.Contains(t, text, "fetch available: true")
	assert.Contains(t, text, "Result: module test completed")
}

func TestModuleConfiguration_DisabledModules(t *testing.T) {
	// Test with timers enabled
	config := ModuleConfig{
		EnabledModules: []string{"timers"},
	}
	handler := NewJSHandlerWithConfig(config)

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			// Test timers availability
			const timersAvailable = typeof require !== 'undefined';
			const fetchAvailable = typeof fetch !== 'undefined';
			
			console.log("timers available:", timersAvailable);
			console.log("fetch available:", fetchAvailable);
			
			"disabled test completed";
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text

	// Should have enabled modules
	assert.Contains(t, text, "Result: disabled test completed")
}

func TestModuleConfiguration_NoConsole(t *testing.T) {
	// Test with basic execution - console.log should work in the runtime
	config := ModuleConfig{
		EnabledModules: []string{}, // No modules enabled
	}
	handler := NewJSHandlerWithConfig(config)

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			// Basic math should work
			const result = 2 + 3;
			console.log("Math works:", result);
			result;
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text

	// Should have the result and console output
	assert.Contains(t, text, "Result: 5")
	assert.Contains(t, text, "Math works: 5")
}

func TestNewJSServerWithConfig(t *testing.T) {
	config := ModuleConfig{
		EnabledModules: []string{"http", "fetch"},
	}

	server, err := NewJSServerWithConfig(config)
	require.NoError(t, err)
	assert.NotNil(t, server)

	// The server should be created successfully with the config
	// We can't easily test the internal configuration without exposing it,
	// but we can verify it doesn't error
}

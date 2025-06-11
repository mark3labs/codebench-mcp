package jsserver

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleConfiguration_EnabledModules(t *testing.T) {
	// Test with only console and timers enabled
	config := ModuleConfig{
		EnabledModules: []string{"console", "timers"},
	}
	handler := NewJSHandlerWithConfig(config)

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			// These should work
			console.log("Console works");
			setTimeout(() => console.log("Timer works"), 100);
			
			// These should not be available
			const fsAvailable = typeof fs !== 'undefined';
			const httpAvailable = typeof http !== 'undefined';
			const fetchAvailable = typeof fetch !== 'undefined';
			const processAvailable = typeof process !== 'undefined';
			
			console.log("fs available:", fsAvailable);
			console.log("http available:", httpAvailable);
			console.log("fetch available:", fetchAvailable);
			console.log("process available:", processAvailable);
			
			"module test completed";
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text

	// Should have console and timers
	assert.Contains(t, text, "Console works")

	// Should not have other modules
	assert.Contains(t, text, "fs available: false")
	assert.Contains(t, text, "http available: false")
	assert.Contains(t, text, "fetch available: false")
	assert.Contains(t, text, "process available: false")
	assert.Contains(t, text, "Result: module test completed")
}

func TestModuleConfiguration_DisabledModules(t *testing.T) {
	// Test with all modules except fs and http
	config := ModuleConfig{
		EnabledModules: []string{"console", "fetch", "timers", "process"},
	}
	handler := NewJSHandlerWithConfig(config)

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			// These should work
			console.log("Console works");
			console.log("fetch available:", typeof fetch !== 'undefined');
			console.log("process available:", typeof process !== 'undefined');
			
			// These should not be available
			console.log("fs available:", typeof fs !== 'undefined');
			console.log("http available:", typeof http !== 'undefined');
			
			"disabled test completed";
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text

	// Should have enabled modules
	assert.Contains(t, text, "Console works")
	assert.Contains(t, text, "fetch available: true")
	assert.Contains(t, text, "process available: true")

	// Should not have disabled modules
	assert.Contains(t, text, "fs available: false")
	assert.Contains(t, text, "http available: false")
	assert.Contains(t, text, "Result: disabled test completed")
}

func TestModuleConfiguration_NoConsole(t *testing.T) {
	// Test with console disabled - should still work but no console output
	config := ModuleConfig{
		EnabledModules: []string{"timers"},
	}
	handler := NewJSHandlerWithConfig(config)

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			// This should fail since console is not available
			try {
				console.log("This should not work");
			} catch (e) {
				// Expected
			}
			
			// But this should work
			const result = 2 + 3;
			result;
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text

	// Should have the result but no console output
	assert.Contains(t, text, "Result: 5")
	assert.NotContains(t, text, "This should not work")
}

func TestNewJSServerWithConfig(t *testing.T) {
	config := ModuleConfig{
		EnabledModules: []string{"console", "fs"},
	}

	server, err := NewJSServerWithConfig(config)
	require.NoError(t, err)
	assert.NotNil(t, server)

	// The server should be created successfully with the config
	// We can't easily test the internal configuration without exposing it,
	// but we can verify it doesn't error
}

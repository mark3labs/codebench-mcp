package jsserver_test

import (
	"context"
	"testing"

	"github.com/mark3labs/codebench-mcp/jsserver"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInProcessTransport(t *testing.T) {
	// Create the JS server
	jsServer, err := jsserver.NewJSServer()
	require.NoError(t, err)

	// Create an in-process client
	mcpClient, err := client.NewInProcessClient(jsServer)
	require.NoError(t, err)
	defer mcpClient.Close()

	// Start the client
	err = mcpClient.Start(context.Background())
	require.NoError(t, err)

	// Initialize the client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}
	result, err := mcpClient.Initialize(context.Background(), initRequest)
	require.NoError(t, err)
	assert.Equal(t, "javascript-executor", result.ServerInfo.Name)

	// List tools to verify executeJS is available
	toolsResult, err := mcpClient.ListTools(context.Background(), mcp.ListToolsRequest{})
	require.NoError(t, err)

	var executeJSTool *mcp.Tool
	for _, tool := range toolsResult.Tools {
		if tool.Name == "executeJS" {
			executeJSTool = &tool
			break
		}
	}
	require.NotNil(t, executeJSTool, "executeJS tool not found")
	assert.Equal(t, "executeJS", executeJSTool.Name)

	// Test calling the executeJS tool
	callRequest := mcp.CallToolRequest{}
	callRequest.Params.Name = "executeJS"
	callRequest.Params.Arguments = map[string]any{
		"code": `
			const message = "Hello from in-process MCP!";
			console.log(message);
			const result = 2 + 3;
			console.log("Calculation:", result);
			result;
		`,
	}

	callResult, err := mcpClient.CallTool(context.Background(), callRequest)
	require.NoError(t, err)
	assert.False(t, callResult.IsError)
	assert.Len(t, callResult.Content, 1)

	text := callResult.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Hello from in-process MCP!")
	assert.Contains(t, text, "Calculation: 5")
	assert.Contains(t, text, "Result: 5")
}

func TestInProcessTransport_ErrorHandling(t *testing.T) {
	// Create the JS server
	jsServer, err := jsserver.NewJSServer()
	require.NoError(t, err)

	// Create an in-process client
	mcpClient, err := client.NewInProcessClient(jsServer)
	require.NoError(t, err)
	defer mcpClient.Close()

	// Start and initialize the client
	err = mcpClient.Start(context.Background())
	require.NoError(t, err)

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}
	_, err = mcpClient.Initialize(context.Background(), initRequest)
	require.NoError(t, err)

	// Test error handling with invalid JavaScript
	callRequest := mcp.CallToolRequest{}
	callRequest.Params.Name = "executeJS"
	callRequest.Params.Arguments = map[string]any{
		"code": `throw new Error("Test error from in-process client");`,
	}

	callResult, err := mcpClient.CallTool(context.Background(), callRequest)
	require.NoError(t, err)
	assert.True(t, callResult.IsError)
	assert.Len(t, callResult.Content, 1)

	text := callResult.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Test error from in-process client")
}

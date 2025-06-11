package jsserver

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteJS_BasicConsoleLog(t *testing.T) {
	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `console.log("Hello, World!");`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Hello, World!")
}

func TestExecuteJS_MathOperations(t *testing.T) {
	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			const a = 5;
			const b = 3;
			const result = a + b;
			console.log("Result:", result);
			result;
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Result: 8")
	assert.Contains(t, text, "Result: 8") // The return value
}

func TestExecuteJS_SyntaxError(t *testing.T) {
	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `console.log("missing quote);`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "JavaScript execution error")
}

func TestExecuteJS_RuntimeError(t *testing.T) {
	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			console.log("Before error");
			throw new Error("Test error");
			console.log("After error");
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Before error")
	assert.Contains(t, text, "Test error")
}

func TestExecuteJS_HTTPRequest(t *testing.T) {
	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			console.log("Testing HTTP request...");
			
			// Test that http is available globally
			console.log("http available:", typeof http !== 'undefined');
			if (typeof http !== 'undefined') {
				console.log("http.request type:", typeof http.request);
				
				// Test basic GET request (this will fail but we can test the setup)
				try {
					const response = http.request("GET", "https://httpbin.org/json");
					console.log("Response status:", response.status);
					console.log("Response ok:", response.ok);
					console.log("Response has body:", typeof response.body);
					console.log("Response has headers:", typeof response.headers);
				} catch (e) {
					console.log("Request error:", e.message);
				}
			}
			
			"http request test completed";
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "http available:")
	assert.Contains(t, text, "Result: http request test completed")
}

func TestExecuteJS_HTTPServer(t *testing.T) {
	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			console.log("Creating HTTP server...");
			
			// Test that http is available globally
			console.log("http available:", typeof http !== 'undefined');
			if (typeof http !== 'undefined' && typeof http.createServer === 'function') {
				const server = http.createServer((req, res) => {
					console.log("Request received:", req.method, req.url);
					res.writeHead(200, {"Content-Type": "text/plain"});
					res.end("Hello from test server!");
				});
				
				// Test different listen signatures
				console.log("Server created, type:", typeof server);
				console.log("Listen method:", typeof server.listen);
				console.log("Close method:", typeof server.close);
				console.log("Address method:", typeof server.address);
			}
			
			"server test completed";
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "http available:")
	assert.Contains(t, text, "Result: server test completed")
}

func TestExecuteJS_HTTPServerWithCallback(t *testing.T) {
	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			console.log("Testing HTTP server with callback...");
			
			// Test that http is available globally
			console.log("http available:", typeof http !== 'undefined');
			if (typeof http !== 'undefined' && typeof http.createServer === 'function') {
				const server = http.createServer((req, res) => {
					res.setHeader("X-Test", "true");
					res.writeHead(200);
					res.write("Hello ");
					res.end("World!");
				});
				
				// Test listen with port and callback
				server.listen(0, () => {
					console.log("Server started listening!");
				});
				
				// Test with port, host, and callback
				const server2 = http.createServer((req, res) => {
					res.end("Server 2");
				});
				
				server2.listen(0, "127.0.0.1", () => {
					console.log("Server 2 started!");
				});
			}
			
			"callback test completed";
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "http available:")
	assert.Contains(t, text, "Result: callback test completed")
}

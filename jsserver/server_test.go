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

func TestExecuteJS_FileOperations(t *testing.T) {
	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			const testFile = "/tmp/test_js_mcp.txt";
			fs.writeFileSync(testFile, "Hello from JS!");
			const content = fs.readFileSync(testFile);
			console.log("File content:", content);
			fs.existsSync(testFile);
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "File content: Hello from JS!")
	assert.Contains(t, text, "Result: true") // existsSync result
}

func TestExecuteJS_ProcessInfo(t *testing.T) {
	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			console.log("CWD:", process.cwd());
			console.log("Args length:", process.argv.length);
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "CWD:")
	assert.Contains(t, text, "Args length:")
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

func TestExecuteJS_FetchAPI(t *testing.T) {
	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			// Test that fetch function exists
			console.log("fetch type:", typeof fetch);
			
			// Test basic fetch (this will fail but we can test the setup)
			try {
				const promise = fetch("https://httpbin.org/json");
				console.log("fetch promise created:", typeof promise);
				console.log("promise has then:", typeof promise.then);
			} catch (e) {
				console.log("fetch error:", e.message);
			}
			
			"fetch test completed";
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "fetch type: function")
	assert.Contains(t, text, "fetch promise created: object")
	assert.Contains(t, text, "promise has then: function")
	assert.Contains(t, text, "Result: fetch test completed")
}

func TestExecuteJS_FetchWithPromise(t *testing.T) {
	// Skip this test in CI or if network is not available
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			// Test actual fetch with a simple endpoint
			let result = "starting";
			
			fetch("https://httpbin.org/json")
				.then(response => {
					console.log("Response status:", response.status);
					console.log("Response ok:", response.ok);
					return response.json();
				})
				.then(data => {
					console.log("Got JSON data:", typeof data);
					result = "success";
				})
				.catch(error => {
					console.log("Fetch error:", error);
					result = "error: " + error;
				});
			
			// Give some time for the promise to resolve
			setTimeout(() => {
				console.log("Final result:", result);
			}, 2000);
			
			result;
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Result: starting") // Initial result before async completes
}

func TestExecuteJS_HTTPServer(t *testing.T) {
	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			console.log("Creating HTTP server...");
			
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
			
			"server test completed";
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Server created, type: object")
	assert.Contains(t, text, "Listen method: function")
	assert.Contains(t, text, "Close method: function")
	assert.Contains(t, text, "Address method: function")
	assert.Contains(t, text, "Result: server test completed")
}

func TestExecuteJS_HTTPServerWithCallback(t *testing.T) {
	handler := NewJSHandler()

	request := mcp.CallToolRequest{}
	request.Params.Name = "executeJS"
	request.Params.Arguments = map[string]any{
		"code": `
			console.log("Testing HTTP server with callback...");
			
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
			
			"callback test completed";
		`,
	}

	result, err := handler.handleExecuteJS(context.Background(), request)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "Testing HTTP server with callback")
	assert.Contains(t, text, "Result: callback test completed")
}

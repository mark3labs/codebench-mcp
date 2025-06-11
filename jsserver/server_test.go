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

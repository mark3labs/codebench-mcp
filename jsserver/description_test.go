package jsserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildToolDescription(t *testing.T) {
	tests := []struct {
		name            string
		enabledModules  []string
		expectedContent []string
		notExpected     []string
	}{
		{
			name:           "All modules enabled",
			enabledModules: []string{"http", "fetch", "timers", "buffer", "crypto"},
			expectedContent: []string{
				"ski runtime",
				"Node.js-like APIs",
				"Available modules:",
				"• http: HTTP server creation and management",
				"• fetch: Modern fetch API with Request, Response, Headers, FormData",
				"• timers: setTimeout, setInterval, clearTimeout, clearInterval",
				"• buffer: Buffer, Blob, File APIs for binary data handling",
				"• crypto: Cryptographic functions (hashing, encryption, HMAC)",
				"Example usage:",
			},
		},
		{
			name:           "Only http and fetch",
			enabledModules: []string{"http", "fetch"},
			expectedContent: []string{
				"ski runtime",
				"Available modules:",
				"• http: HTTP server creation and management",
				"• fetch: Modern fetch API with Request, Response, Headers, FormData",
				"Example usage:",
			},
			notExpected: []string{
				"• timers:",
				"• buffer:",
				"• crypto:",
			},
		},
		{
			name:           "No modules enabled",
			enabledModules: []string{},
			expectedContent: []string{
				"ski runtime",
				"No modules are currently enabled",
				"Only basic JavaScript execution is available",
			},
			notExpected: []string{
				"Available modules:",
				"Usage:",
			},
		},
		{
			name:           "Only http enabled",
			enabledModules: []string{"http"},
			expectedContent: []string{
				"Available modules:",
				"• http: HTTP server creation and management",
			},
			notExpected: []string{
				"• fetch:",
				"• timers:",
				"• buffer:",
				"• crypto:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			description := buildToolDescription(tt.enabledModules)

			// Check expected content
			for _, expected := range tt.expectedContent {
				assert.Contains(t, description, expected, "Description should contain: %s", expected)
			}

			// Check content that should not be present
			for _, notExpected := range tt.notExpected {
				assert.NotContains(t, description, notExpected, "Description should not contain: %s", notExpected)
			}
		})
	}
}

func TestToolDescriptionDynamicUpdate(t *testing.T) {
	// Test that different configurations produce different descriptions
	config1 := ModuleConfig{EnabledModules: []string{"http", "fetch"}}
	config2 := ModuleConfig{EnabledModules: []string{"timers"}}

	server1, err := NewJSServerWithConfig(config1)
	assert.NoError(t, err)
	assert.NotNil(t, server1)

	server2, err := NewJSServerWithConfig(config2)
	assert.NoError(t, err)
	assert.NotNil(t, server2)

	// The descriptions should be different
	desc1 := buildToolDescription(config1.EnabledModules)
	desc2 := buildToolDescription(config2.EnabledModules)

	assert.NotEqual(t, desc1, desc2, "Different module configurations should produce different descriptions")

	// Config1 should mention http and fetch
	assert.Contains(t, desc1, "• http:")
	assert.Contains(t, desc1, "• fetch:")
	assert.NotContains(t, desc1, "• timers:")

	// Config2 should mention timers
	assert.Contains(t, desc2, "• timers:")
	assert.NotContains(t, desc2, "• http:")
	assert.NotContains(t, desc2, "• fetch:")
}

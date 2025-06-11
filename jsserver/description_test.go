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
			enabledModules: []string{"console", "fs", "http", "fetch", "timers", "process", "require"},
			expectedContent: []string{
				"simplified JavaScript VM (goja)",
				"NOT a full Node.js environment",
				"📦 Available modules:",
				"• console: Console logging",
				"• fs: File system operations",
				"• http: HTTP server creation",
				"• fetch: HTTP client requests",
				"• timers: Timer functions",
				"• process: Process information",
				"• require: Module loading system",
				"💡 Usage:",
				"'undefined' errors",
			},
		},
		{
			name:           "Only console and timers",
			enabledModules: []string{"console", "timers"},
			expectedContent: []string{
				"simplified JavaScript VM (goja)",
				"📦 Available modules:",
				"• console: Console logging",
				"• timers: Timer functions",
				"💡 Usage:",
			},
			notExpected: []string{
				"• fs:",
				"• http:",
				"• fetch:",
				"• process:",
				"• require:",
			},
		},
		{
			name:           "No modules enabled",
			enabledModules: []string{},
			expectedContent: []string{
				"simplified JavaScript VM (goja)",
				"NOT a full Node.js environment",
				"⚠️  No modules are currently enabled",
				"Only basic JavaScript execution is available",
			},
			notExpected: []string{
				"📦 Available modules:",
				"💡 Usage:",
			},
		},
		{
			name:           "Only fetch enabled",
			enabledModules: []string{"fetch"},
			expectedContent: []string{
				"📦 Available modules:",
				"• fetch: HTTP client requests (fetch API with Promise support for GET/POST/etc)",
			},
			notExpected: []string{
				"• console:",
				"• fs:",
				"• http:",
				"• timers:",
				"• process:",
				"• require:",
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
	config1 := ModuleConfig{EnabledModules: []string{"console", "fs"}}
	config2 := ModuleConfig{EnabledModules: []string{"fetch", "timers"}}

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

	// Config1 should mention console and fs
	assert.Contains(t, desc1, "• console:")
	assert.Contains(t, desc1, "• fs:")
	assert.NotContains(t, desc1, "• fetch:")
	assert.NotContains(t, desc1, "• timers:")

	// Config2 should mention fetch and timers
	assert.Contains(t, desc2, "• fetch:")
	assert.Contains(t, desc2, "• timers:")
	assert.NotContains(t, desc2, "• console:")
	assert.NotContains(t, desc2, "• fs:")
}

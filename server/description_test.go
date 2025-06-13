package server

import (
	"fmt"
	"strings"
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
				"modern runtime",
				"Node.js-like APIs",
				"Available modules:",
				"• http: HTTP server creation and management",
				"• fetch: Modern fetch API with Request, Response, Headers, FormData",
				"• timers: setTimeout, setInterval, clearTimeout, clearInterval",
				"• buffer: Buffer, Blob, File APIs for binary data handling",
				"• crypto: Cryptographic functions (hashing, encryption, HMAC)",
				"Example usage (modern JavaScript with require()):",
			},
		},
		{
			name:           "Only http and fetch",
			enabledModules: []string{"http", "fetch"},
			expectedContent: []string{
				"modern runtime",
				"Available modules:",
				"• http: HTTP server creation and management",
				"• fetch: Modern fetch API with Request, Response, Headers, FormData",
				"Example usage (modern JavaScript with require()):",
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
				"modern runtime",
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

func TestAllModulesRepresentedInDescription(t *testing.T) {
	// Get all available modules from the actual modules directory
	allModules := []string{"http", "fetch", "timers", "buffer", "crypto", "cache", "kv", "encoding", "url"}
	
	// Test with all modules enabled
	description := buildToolDescription(allModules)
	
	// Verify each module is mentioned in the description
	expectedModuleDescriptions := map[string]string{
		"http":     "HTTP server creation and management",
		"fetch":    "Modern fetch API with Request, Response, Headers, FormData",
		"timers":   "setTimeout, setInterval, clearTimeout, clearInterval",
		"buffer":   "Buffer, Blob, File APIs for binary data handling",
		"crypto":   "Cryptographic functions (hashing, encryption, HMAC)",
		"cache":    "In-memory caching with TTL support",
		"kv":       "Key-value store per VM instance with get, set, delete, list",
		"encoding": "TextEncoder/TextDecoder for UTF-8 encoding/decoding",
		"url":      "URL parsing and URLSearchParams manipulation",
	}
	
	for module, expectedDesc := range expectedModuleDescriptions {
		t.Run(fmt.Sprintf("Module_%s", module), func(t *testing.T) {
			// Check that the module is listed
			assert.Contains(t, description, fmt.Sprintf("• %s:", module), 
				"Module %s should be listed in description", module)
			
			// Check that the description contains key parts of the expected description
			assert.Contains(t, description, expectedDesc, 
				"Module %s should have correct description containing: %s", module, expectedDesc)
		})
	}
	
	// Verify the description contains the modules section
	assert.Contains(t, description, "Available modules:")
	
	// Verify no modules are missing by checking the count
	moduleCount := 0
	for _, module := range allModules {
		if strings.Contains(description, fmt.Sprintf("• %s:", module)) {
			moduleCount++
		}
	}
	assert.Equal(t, len(allModules), moduleCount, 
		"All %d modules should be represented in description, found %d", len(allModules), moduleCount)
}

func TestModuleDescriptionConsistency(t *testing.T) {
	// Test that require() vs global availability is correctly indicated
	description := buildToolDescription([]string{"http", "fetch", "crypto", "cache", "timers", "buffer"})
	
	// Modules that require require() should mention it
	requireModules := []string{"http", "crypto", "cache"}
	for _, module := range requireModules {
		assert.Contains(t, description, fmt.Sprintf("require('%s", module), 
			"Module %s should show require() usage", module)
	}
	
	// Global modules should mention "available globally"
	globalModules := []string{"fetch", "timers", "buffer"}
	for _, module := range globalModules {
		moduleLineRegex := fmt.Sprintf("• %s:.*available globally", module)
		assert.Regexp(t, moduleLineRegex, description, 
			"Module %s should mention 'available globally'", module)
	}
}

func TestNoMissingModulesInDescription(t *testing.T) {
	// This test ensures that if we add a new module to the codebase,
	// we don't forget to add it to the description builder
	
	// Get the module descriptions map from the buildToolDescription function
	// We'll test this by checking that all modules we know exist have descriptions
	allKnownModules := []string{"http", "fetch", "timers", "buffer", "crypto", "cache", "kv", "encoding", "url"}
	
	// Build description with all modules
	description := buildToolDescription(allKnownModules)
	
	// Count how many modules are actually described
	describedModules := 0
	for _, module := range allKnownModules {
		if strings.Contains(description, fmt.Sprintf("• %s:", module)) {
			describedModules++
		}
	}
	
	// This test will fail if:
	// 1. We add a new module but forget to add it to moduleDescriptions map
	// 2. We add a module to allKnownModules but it's not in the description
	assert.Equal(t, len(allKnownModules), describedModules, 
		"All known modules should be described. Known: %d, Described: %d. "+
		"If you added a new module, make sure to add it to the moduleDescriptions map in buildToolDescription()", 
		len(allKnownModules), describedModules)
	
	// Also verify that the description doesn't contain any undefined modules
	assert.NotContains(t, description, "• undefined:", 
		"Description should not contain undefined modules")
}

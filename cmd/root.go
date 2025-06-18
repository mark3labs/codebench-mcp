package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/mark3labs/codebench-mcp/internal/logger"
	"github.com/mark3labs/codebench-mcp/server"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var (
	enabledModules  []string
	disabledModules []string
	debugMode       bool
	executionTimeout int
)

// Available modules
var availableModules = []string{
	"http",
	"fetch",
	"timers",
	"buffer",
	"kv",
	"crypto",
	"encoding",
	"url",
	"cache",
	// TODO: Add these as they're implemented
	// "dom",
	// "ext",
	// "html",
	// "signal",
	// "stream",
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "codebench-mcp",
	Short: "JavaScript Executor MCP Server",
	Long: `A Model Context Protocol (MCP) server that provides JavaScript execution capabilities 
with a modern runtime including http, fetch, timers, buffer, crypto, and other modules.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize logger first
		logger.Init(debugMode)

		logger.Debug("Starting codebench-mcp server", "debug", debugMode)

		// Validate module configuration
		if len(enabledModules) > 0 && len(disabledModules) > 0 {
			logger.Fatal("--enabled-modules and --disabled-modules are mutually exclusive")
		}

		// Determine which modules to enable
		var modulesToEnable []string
		if len(enabledModules) > 0 {
			// Only enable specified modules
			for _, module := range enabledModules {
				if !slices.Contains(availableModules, module) {
					logger.Fatal("unknown module", "module", module, "available", strings.Join(availableModules, ", "))
				}
			}
			modulesToEnable = enabledModules
		} else if len(disabledModules) > 0 {
			// Enable all modules except disabled ones
			for _, module := range disabledModules {
				if !slices.Contains(availableModules, module) {
					logger.Fatal("unknown module", "module", module, "available", strings.Join(availableModules, ", "))
				}
			}
			for _, module := range availableModules {
				if !slices.Contains(disabledModules, module) {
					modulesToEnable = append(modulesToEnable, module)
				}
			}
		} else {
			// Enable default modules (same as NewJSHandler default)
			modulesToEnable = []string{"http", "fetch", "timers", "buffer", "kv", "crypto", "encoding", "url", "cache"}
		}

		logger.Debug("Module configuration", "enabled", modulesToEnable)

		// Create server with module configuration
		config := server.ModuleConfig{
			EnabledModules: modulesToEnable,
			ExecutionTimeout: time.Duration(executionTimeout) * time.Second,
		}

		jss, err := server.NewJSServerWithConfig(config)
		if err != nil {
			logger.Fatal("Failed to create server", "error", err)
		}

		logger.Info("Starting MCP server", "modules", modulesToEnable)

		// Serve requests
		if err := mcpserver.ServeStdio(jss); err != nil {
			logger.Fatal("Server error", "error", err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringSliceVar(&enabledModules, "enabled-modules", nil,
		fmt.Sprintf("Comma-separated list of modules to enable. Available: %s",
			strings.Join(availableModules, ", ")))
	rootCmd.Flags().StringSliceVar(&disabledModules, "disabled-modules", nil,
		fmt.Sprintf("Comma-separated list of modules to disable. Available: %s",
			strings.Join(availableModules, ", ")))
	rootCmd.Flags().BoolVar(&debugMode, "debug", false,
		"Enable debug logging (outputs to stderr)")
	rootCmd.Flags().IntVar(&executionTimeout, "execution-timeout", 300,
		"JavaScript execution timeout in seconds (default: 300 = 5 minutes)")

	rootCmd.MarkFlagsMutuallyExclusive("enabled-modules", "disabled-modules")
}

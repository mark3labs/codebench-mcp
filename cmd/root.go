package cmd

import (
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/mark3labs/codebench-mcp/jsserver"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var (
	enabledModules  []string
	disabledModules []string
)

// Available modules
var availableModules = []string{
	"http",
	"fetch",
	"timers",
	"buffer",
	"cache",
	"crypto",
	"dom",
	"encoding",
	"ext",
	"html",
	"signal",
	"stream",
	"url",
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "codebench-mcp",
	Short: "JavaScript Executor MCP Server",
	Long: `A Model Context Protocol (MCP) server that provides JavaScript execution capabilities 
with ski runtime including http, fetch, timers, buffer, crypto, and other modules.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Validate module configuration
		if len(enabledModules) > 0 && len(disabledModules) > 0 {
			log.Fatal("Error: --enabled-modules and --disabled-modules are mutually exclusive")
		}

		// Determine which modules to enable
		var modulesToEnable []string
		if len(enabledModules) > 0 {
			// Only enable specified modules
			for _, module := range enabledModules {
				if !slices.Contains(availableModules, module) {
					log.Fatalf("Error: unknown module '%s'. Available modules: %s",
						module, strings.Join(availableModules, ", "))
				}
			}
			modulesToEnable = enabledModules
		} else if len(disabledModules) > 0 {
			// Enable all modules except disabled ones
			for _, module := range disabledModules {
				if !slices.Contains(availableModules, module) {
					log.Fatalf("Error: unknown module '%s'. Available modules: %s",
						module, strings.Join(availableModules, ", "))
				}
			}
			for _, module := range availableModules {
				if !slices.Contains(disabledModules, module) {
					modulesToEnable = append(modulesToEnable, module)
				}
			}
		} else {
			// Enable default modules (same as NewJSHandler default)
			modulesToEnable = []string{"http", "fetch", "timers", "buffer", "crypto"}
		}

		// Create server with module configuration
		config := jsserver.ModuleConfig{
			EnabledModules: modulesToEnable,
		}

		jss, err := jsserver.NewJSServerWithConfig(config)
		if err != nil {
			log.Fatalf("Failed to create server: %v", err)
		}

		// Serve requests
		if err := server.ServeStdio(jss); err != nil {
			log.Fatalf("Server error: %v", err)
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

	rootCmd.MarkFlagsMutuallyExclusive("enabled-modules", "disabled-modules")
}

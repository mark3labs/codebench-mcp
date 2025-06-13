package logger

import (
	"os"

	"github.com/charmbracelet/log"
)

var (
	// Logger is the global logger instance
	Logger *log.Logger
	// DebugEnabled tracks if debug logging is enabled
	DebugEnabled bool
)

// Init initializes the global logger with the specified debug level
func Init(debug bool) {
	DebugEnabled = debug

	// Create logger that outputs to stderr (stdin/stdout reserved for MCP)
	Logger = log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    debug, // Show caller info in debug mode
		ReportTimestamp: true,
		TimeFormat:      "15:04:05",
		Prefix:          "codebench-mcp",
	})

	// Set log level based on debug flag
	if debug {
		Logger.SetLevel(log.DebugLevel)
	} else {
		Logger.SetLevel(log.InfoLevel)
	}
}

// Debug logs a debug message (only if debug is enabled)
func Debug(msg interface{}, keyvals ...interface{}) {
	if Logger != nil {
		Logger.Debug(msg, keyvals...)
	}
}

// Info logs an info message
func Info(msg interface{}, keyvals ...interface{}) {
	if Logger != nil {
		Logger.Info(msg, keyvals...)
	}
}

// Warn logs a warning message
func Warn(msg interface{}, keyvals ...interface{}) {
	if Logger != nil {
		Logger.Warn(msg, keyvals...)
	}
}

// Error logs an error message
func Error(msg interface{}, keyvals ...interface{}) {
	if Logger != nil {
		Logger.Error(msg, keyvals...)
	}
}

// Fatal logs a fatal message and exits
func Fatal(msg interface{}, keyvals ...interface{}) {
	if Logger != nil {
		Logger.Fatal(msg, keyvals...)
	} else {
		// Fallback if logger not initialized
		log.Fatal(msg, keyvals...)
	}
}

// GetLogger returns the global logger instance (for use in modules)
func GetLogger() *log.Logger {
	return Logger
}

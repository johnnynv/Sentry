package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	// Test with verbose logging
	logger := NewLogger(true)
	if logger == nil {
		t.Error("NewLogger() returned nil")
	}

	// Test with non-verbose logging
	logger = NewLogger(false)
	if logger == nil {
		t.Error("NewLogger() returned nil")
	}
}

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelDebug, "DEBUG"},
		{LogLevelInfo, "INFO"},
		{LogLevelWarn, "WARN"},
		{LogLevelError, "ERROR"},
		{LogLevelFatal, "FATAL"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("LogLevel.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLoggerStructuredLogging(t *testing.T) {
	logger := NewLogger(true)

	// Test InfoS
	logger.InfoS("Test info message", "key1", "value1", "key2", "value2")

	// Test WarnS
	logger.WarnS("Test warn message", "warning", "test")

	// Test ErrorS
	logger.ErrorS("Test error message", "error", "test")

	// Test DebugS
	logger.DebugS("Test debug message", "debug", "test")

	// Test with no key-value pairs
	logger.InfoS("Test message with no pairs")

	// Test with odd number of arguments
	logger.InfoS("Test message with odd args", "key1", "value1", "key2")
}

func TestLoggerBasicLogging(t *testing.T) {
	logger := NewLogger(false)

	// Test all log levels
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warning message")
	logger.Error("Error message")

	// Note: We don't test Fatal() as it would exit the program
}

func TestLoggerRepositoryOperations(t *testing.T) {
	logger := NewLogger(true)

	// Test repository check logging
	logger.LogRepositoryCheck("test-repo:main", true, "abc123def456", "Test Author")
	logger.LogRepositoryCheck("test-repo:main", false, "abcdefghijklmnop", "")

	// Test deployment logging
	logger.LogDeploymentStart("test-repo", 3)
	logger.LogDeploymentSuccess("test-repo", 3)
	logger.LogDeploymentFailure("test-repo", fmt.Errorf("test error"))

	// Test retry logging
	logger.LogRetryAttempt("API call", 1, 3, fmt.Errorf("connection failed"))

	// Test cleanup logging
	logger.LogCleanup("/tmp/test", true)
	logger.LogCleanup("/tmp/test", false)

	// Test API call logging
	logger.LogAPICall("GET", "https://api.github.com/repos/owner/repo", 200, 150*time.Millisecond)
	logger.LogAPICall("GET", "https://api.github.com/repos/owner/repo", 404, 50*time.Millisecond)

	// Test group deployment logging
	logger.LogGroupDeploymentSuccess("test-group", 2, "5s")
	logger.LogGroupDeploymentFailure("test-group", fmt.Errorf("deployment failed"))
}

func TestLoggerStructuredLogWithInvalidPairs(t *testing.T) {
	logger := NewLogger(true)

	// Test with nil values
	logger.InfoS("Test with nil", "key1", nil, "key2", "value2")

	// Test with various types
	logger.InfoS("Test with types",
		"string", "value",
		"int", 42,
		"bool", true,
		"float", 3.14,
		"duration", 5*time.Second)
}

func TestInitializeLogger(t *testing.T) {
	// Test initializing with verbose
	InitializeLogger(true)
	if AppLogger == nil {
		t.Error("InitializeLogger() did not set AppLogger")
	}

	// Test initializing with non-verbose
	InitializeLogger(false)
	if AppLogger == nil {
		t.Error("InitializeLogger() did not set AppLogger")
	}
}

func TestLoggerComplexScenarios(t *testing.T) {
	logger := NewLogger(true)

	// Test logging with special characters
	logger.InfoS("Message with special chars: @#$%^&*()", "key", "value with spaces and symbols: !@#")

	// Test logging with empty strings
	logger.InfoS("", "empty_key", "", "key", "value")

	// Test logging with very long strings
	longString := strings.Repeat("a", 1000)
	logger.InfoS("Long message test", "long_key", longString)

	// Test rapid sequential logging
	for i := 0; i < 10; i++ {
		logger.InfoS("Rapid log", "iteration", i)
	}
}

func TestLoggerEdgeCases(t *testing.T) {
	logger := NewLogger(false) // Test with non-verbose logger

	// Test with empty message
	logger.InfoS("")

	// Test with only message, no pairs
	logger.InfoS("Just a message")

	// Test structured log with nil logger (should not panic)
	// A nil logger should not be called to avoid panics

	// Test with numeric keys (will be converted to strings)
	logger.InfoS("Numeric keys test", 123, "value1", 456.78, "value2")
}

func TestLoggerFormatting(t *testing.T) {
	logger := NewLogger(true)

	// Test various formatting scenarios that could cause issues
	logger.InfoS("Test with format chars %s %d %v", "key", "value")
	logger.InfoS("Test with newlines\nand\ttabs", "multiline", "value\nwith\nnewlines")

	// Test with boolean and numeric values
	logger.InfoS("Boolean and numbers",
		"success", true,
		"count", 42,
		"percentage", 85.5,
		"negative", -10)
}

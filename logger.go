package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel represents different log levels
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// String returns string representation of log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging functionality
type Logger struct {
	level   LogLevel
	verbose bool
	logger  *log.Logger
}

// NewLogger creates a new logger instance
func NewLogger(verbose bool) *Logger {
	level := LogLevelInfo
	if verbose {
		level = LogLevelDebug
	}

	return &Logger{
		level:   level,
		verbose: verbose,
		logger:  log.New(os.Stdout, "", 0),
	}
}

// logf formats and logs a message at the specified level
func (l *Logger) logf(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	prefix := fmt.Sprintf("[%s] %s: ", timestamp, level.String())
	message := fmt.Sprintf(format, args...)

	l.logger.Printf("%s%s", prefix, message)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.logf(LogLevelDebug, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.logf(LogLevelInfo, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.logf(LogLevelWarn, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.logf(LogLevelError, format, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.logf(LogLevelFatal, format, args...)
	os.Exit(1)
}

// LogRepositoryCheck logs repository monitoring activity
func (l *Logger) LogRepositoryCheck(repoKey string, success bool, commitSHA string, author string) {
	if success {
		l.Info("Repository %s check successful - Latest commit: %s by %s", repoKey, commitSHA[:8], author)
	} else {
		l.Warn("Repository %s check failed", repoKey)
	}
}

// LogDeploymentStart logs deployment start
func (l *Logger) LogDeploymentStart(repoKey string, filesCount int) {
	l.Info("Starting deployment from %s - %d Tekton files found", repoKey, filesCount)
}

// LogDeploymentSuccess logs successful deployment
func (l *Logger) LogDeploymentSuccess(repoKey string, filesDeployed int) {
	l.Info("Deployment successful: %s - %d files deployed", repoKey, filesDeployed)
}

// LogDeploymentFailure logs deployment failure
func (l *Logger) LogDeploymentFailure(repoKey string, err error) {
	l.Error("Deployment failed: %s - %v", repoKey, err)
}

// LogRetryAttempt logs retry attempts
func (l *Logger) LogRetryAttempt(operation string, attempt int, maxRetries int, err error) {
	l.Warn("Retry %d/%d for %s after error: %v", attempt, maxRetries, operation, err)
}

// LogCleanup logs cleanup operations
func (l *Logger) LogCleanup(path string, success bool) {
	if success {
		l.Debug("Cleanup successful: %s", path)
	} else {
		l.Warn("Cleanup failed: %s", path)
	}
}

// LogAPICall logs API call information
func (l *Logger) LogAPICall(service string, url string, statusCode int, duration time.Duration) {
	if statusCode >= 200 && statusCode < 300 {
		l.Debug("API call successful: %s %s - %d (%v)", service, url, statusCode, duration)
	} else {
		l.Warn("API call failed: %s %s - %d (%v)", service, url, statusCode, duration)
	}
}

// Global logger instance
var AppLogger *Logger

// InitializeLogger initializes the global logger
func InitializeLogger(verbose bool) {
	AppLogger = NewLogger(verbose)
}

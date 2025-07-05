package logger

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/piske-alex/margin-offer-system/types"
)

// Logger implements the types.Logger interface using logrus
type Logger struct {
	logger *logrus.Entry
}

// NewLogger creates a new logger instance
func NewLogger(service string) types.Logger {
	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.SetFormatter(&logrus.JSONFormatter{})
	
	// Set log level from environment
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	}
	
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	log.SetLevel(logLevel)
	
	return &Logger{
		logger: log.WithField("service", service),
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.logWithFields(logrus.DebugLevel, msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...interface{}) {
	l.logWithFields(logrus.InfoLevel, msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...interface{}) {
	l.logWithFields(logrus.WarnLevel, msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...interface{}) {
	l.logWithFields(logrus.ErrorLevel, msg, fields...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, fields ...interface{}) {
	l.logWithFields(logrus.FatalLevel, msg, fields...)
}

// WithField returns a new logger with the given field
func (l *Logger) WithField(key string, value interface{}) types.Logger {
	return &Logger{
		logger: l.logger.WithField(key, value),
	}
}

// WithFields returns a new logger with the given fields
func (l *Logger) WithFields(fields map[string]interface{}) types.Logger {
	return &Logger{
		logger: l.logger.WithFields(logrus.Fields(fields)),
	}
}

// logWithFields logs a message with optional key-value pairs
func (l *Logger) logWithFields(level logrus.Level, msg string, fields ...interface{}) {
	entry := l.logger
	
	// Add fields as key-value pairs
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			entry = entry.WithField(key, fields[i+1])
		}
	}
	
	entry.Log(level, msg)
}
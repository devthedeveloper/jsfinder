package utils

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogger_SetLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger(INFO, buf)
	logger.SetLevel(WARN)
	
	// Test that debug messages are not logged at WARN level
	logger.Debug("debug message")
	if strings.Contains(buf.String(), "debug message") {
		t.Error("Debug message should not be logged at WARN level")
	}
	
	// Test that warn messages are logged at WARN level
	buf.Reset()
	logger.Warn("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Error("Warn message should be logged at WARN level")
	}
}

func TestLogger_SetOutput(t *testing.T) {
	logger := NewLogger(INFO, nil)
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)
	
	logger.Info("test message")
	
	if !strings.Contains(buf.String(), "test message") {
		t.Error("Expected log message not found in output")
	}
}

func TestLogger_LogLevels(t *testing.T) {
	tests := []struct {
		name     string
		level    LogLevel
		logFunc  func(*Logger)
		message  string
		expected bool
	}{
		{"Debug at DEBUG level", DEBUG, func(l *Logger) { l.Debug("debug msg") }, "debug msg", true},
		{"Info at DEBUG level", DEBUG, func(l *Logger) { l.Info("info msg") }, "info msg", true},
		{"Warn at DEBUG level", DEBUG, func(l *Logger) { l.Warn("warn msg") }, "warn msg", true},
		{"Error at DEBUG level", DEBUG, func(l *Logger) { l.Error("error msg") }, "error msg", true},
		{"Debug at INFO level", INFO, func(l *Logger) { l.Debug("debug msg") }, "debug msg", false},
		{"Info at INFO level", INFO, func(l *Logger) { l.Info("info msg") }, "info msg", true},
		{"Warn at INFO level", INFO, func(l *Logger) { l.Warn("warn msg") }, "warn msg", true},
		{"Error at INFO level", INFO, func(l *Logger) { l.Error("error msg") }, "error msg", true},
		{"Debug at WARN level", WARN, func(l *Logger) { l.Debug("debug msg") }, "debug msg", false},
		{"Info at WARN level", WARN, func(l *Logger) { l.Info("info msg") }, "info msg", false},
		{"Warn at WARN level", WARN, func(l *Logger) { l.Warn("warn msg") }, "warn msg", true},
		{"Error at WARN level", WARN, func(l *Logger) { l.Error("error msg") }, "error msg", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger := NewLogger(tt.level, buf)
			
			tt.logFunc(logger)
			
			contains := strings.Contains(buf.String(), tt.message)
			if contains != tt.expected {
				t.Errorf("Expected message presence %v, got %v", tt.expected, contains)
			}
		})
	}
}

func TestLogger_WithFields(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger(INFO, buf)
	
	logger.WithFields(map[string]string{
		"key1": "value1",
		"key2": "value2",
	}).Info("test message")
	
	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("Expected log message not found")
	}
	if !strings.Contains(output, "key1=value1") {
		t.Error("Expected field key1=value1 not found")
	}
	if !strings.Contains(output, "key2=value2") {
		t.Error("Expected field key2=value2 not found")
	}
}

func TestLogger_Formatted(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger(INFO, buf)
	
	logger.Infof("Hello %s, number: %d", "world", 123)
	
	if !strings.Contains(buf.String(), "Hello world, number: 123") {
		t.Error("Expected formatted message not found")
	}
}

func TestNewDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger()
	if logger == nil {
		t.Error("NewDefaultLogger should return a non-nil logger")
	}
}
package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// LogLevel represents the logging level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger represents a structured logger
type Logger struct {
	level  LogLevel
	output io.Writer
	logger *log.Logger
}

// NewLogger creates a new logger instance
func NewLogger(level LogLevel, output io.Writer) *Logger {
	if output == nil {
		output = os.Stderr
	}

	return &Logger{
		level:  level,
		output: output,
		logger: log.New(output, "", 0),
	}
}

// NewDefaultLogger creates a logger with default settings
func NewDefaultLogger() *Logger {
	return NewLogger(INFO, os.Stderr)
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// SetOutput sets the output writer
func (l *Logger) SetOutput(output io.Writer) {
	l.output = output
	l.logger.SetOutput(output)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(DEBUG, msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(INFO, msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(WARN, msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(ERROR, msg, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(FATAL, msg, args...)
	os.Exit(1)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.logf(DEBUG, format, args...)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.logf(INFO, format, args...)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.logf(WARN, format, args...)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logf(ERROR, format, args...)
}

// Fatalf logs a formatted fatal message and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.logf(FATAL, format, args...)
	os.Exit(1)
}

// WithField returns a new logger with additional field
func (l *Logger) WithField(key, value string) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: map[string]string{key: value},
	}
}

// WithFields returns a new logger with additional fields
func (l *Logger) WithFields(fields map[string]string) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

func (l *Logger) log(level LogLevel, msg string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelStr := level.String()

	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}

	logMsg := fmt.Sprintf("[%s] %s: %s", timestamp, levelStr, msg)
	l.logger.Println(logMsg)
}

func (l *Logger) logf(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelStr := level.String()
	msg := fmt.Sprintf(format, args...)

	logMsg := fmt.Sprintf("[%s] %s: %s", timestamp, levelStr, msg)
	l.logger.Println(logMsg)
}

// FieldLogger represents a logger with additional fields
type FieldLogger struct {
	logger *Logger
	fields map[string]string
}

// Debug logs a debug message with fields
func (fl *FieldLogger) Debug(msg string, args ...interface{}) {
	fl.log(DEBUG, msg, args...)
}

// Info logs an info message with fields
func (fl *FieldLogger) Info(msg string, args ...interface{}) {
	fl.log(INFO, msg, args...)
}

// Warn logs a warning message with fields
func (fl *FieldLogger) Warn(msg string, args ...interface{}) {
	fl.log(WARN, msg, args...)
}

// Error logs an error message with fields
func (fl *FieldLogger) Error(msg string, args ...interface{}) {
	fl.log(ERROR, msg, args...)
}

// Fatal logs a fatal message with fields and exits
func (fl *FieldLogger) Fatal(msg string, args ...interface{}) {
	fl.log(FATAL, msg, args...)
	os.Exit(1)
}

// WithField adds another field to the logger
func (fl *FieldLogger) WithField(key, value string) *FieldLogger {
	newFields := make(map[string]string)
	for k, v := range fl.fields {
		newFields[k] = v
	}
	newFields[key] = value

	return &FieldLogger{
		logger: fl.logger,
		fields: newFields,
	}
}

func (fl *FieldLogger) log(level LogLevel, msg string, args ...interface{}) {
	if level < fl.logger.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelStr := level.String()

	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}

	// Build fields string
	fieldsStr := ""
	for key, value := range fl.fields {
		if fieldsStr != "" {
			fieldsStr += " "
		}
		fieldsStr += fmt.Sprintf("%s=%s", key, value)
	}

	logMsg := fmt.Sprintf("[%s] %s: %s", timestamp, levelStr, msg)
	if fieldsStr != "" {
		logMsg += fmt.Sprintf(" [%s]", fieldsStr)
	}

	fl.logger.logger.Println(logMsg)
}

// Global logger instance
var defaultLogger = NewDefaultLogger()

// Global logging functions
func Debug(msg string, args ...interface{}) {
	defaultLogger.Debug(msg, args...)
}

func Info(msg string, args ...interface{}) {
	defaultLogger.Info(msg, args...)
}

func Warn(msg string, args ...interface{}) {
	defaultLogger.Warn(msg, args...)
}

func Error(msg string, args ...interface{}) {
	defaultLogger.Error(msg, args...)
}

func Fatal(msg string, args ...interface{}) {
	defaultLogger.Fatal(msg, args...)
}

func Debugf(format string, args ...interface{}) {
	defaultLogger.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	defaultLogger.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	defaultLogger.Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	defaultLogger.Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	defaultLogger.Fatalf(format, args...)
}

// SetGlobalLevel sets the global logger level
func SetGlobalLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}

// SetGlobalOutput sets the global logger output
func SetGlobalOutput(output io.Writer) {
	defaultLogger.SetOutput(output)
}
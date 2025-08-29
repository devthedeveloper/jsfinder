package utils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// ErrorType represents different types of errors
type ErrorType int

const (
	UnknownError ErrorType = iota
	NetworkError
	TimeoutError
	HTTPError
	ParseError
	ConfigError
	ValidationError
	FileError
)

// String returns the string representation of the error type
func (e ErrorType) String() string {
	switch e {
	case NetworkError:
		return "NETWORK_ERROR"
	case TimeoutError:
		return "TIMEOUT_ERROR"
	case HTTPError:
		return "HTTP_ERROR"
	case ParseError:
		return "PARSE_ERROR"
	case ConfigError:
		return "CONFIG_ERROR"
	case ValidationError:
		return "VALIDATION_ERROR"
	case FileError:
		return "FILE_ERROR"
	default:
		return "UNKNOWN_ERROR"
	}
}

// AppError represents a structured application error
type AppError struct {
	Type      ErrorType
	Message   string
	Cause     error
	Context   map[string]interface{}
	Timestamp time.Time
	Retryable bool
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type.String(), e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type.String(), e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether the error is retryable
func (e *AppError) IsRetryable() bool {
	return e.Retryable
}

// WithContext adds context to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewError creates a new application error
func NewError(errType ErrorType, message string, cause error) *AppError {
	return &AppError{
		Type:      errType,
		Message:   message,
		Cause:     cause,
		Context:   make(map[string]interface{}),
		Timestamp: time.Now(),
		Retryable: isRetryableError(errType, cause),
	}
}

// NewNetworkError creates a network error
func NewNetworkError(message string, cause error) *AppError {
	return NewError(NetworkError, message, cause)
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(message string, cause error) *AppError {
	return NewError(TimeoutError, message, cause)
}

// NewHTTPError creates an HTTP error
func NewHTTPError(message string, statusCode int, cause error) *AppError {
	err := NewError(HTTPError, message, cause)
	err.WithContext("status_code", statusCode)
	return err
}

// NewParseError creates a parse error
func NewParseError(message string, cause error) *AppError {
	return NewError(ParseError, message, cause)
}

// NewConfigError creates a config error
func NewConfigError(message string, cause error) *AppError {
	return NewError(ConfigError, message, cause)
}

// NewValidationError creates a validation error
func NewValidationError(message string, cause error) *AppError {
	return NewError(ValidationError, message, cause)
}

// NewFileError creates a file error
func NewFileError(message string, cause error) *AppError {
	return NewError(FileError, message, cause)
}

// isRetryableError determines if an error is retryable
func isRetryableError(errType ErrorType, cause error) bool {
	switch errType {
	case NetworkError, TimeoutError:
		return true
	case HTTPError:
		// HTTP errors are generally retryable for 5xx and 429 status codes
		// The actual status code should be stored in the error context
		return true
	default:
		return false
	}
}

// IsNetworkError checks if an error is a network error
func IsNetworkError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == NetworkError
	}
	
	// Check for common network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	
	// Check for DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	
	return false
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == TimeoutError
	}
	
	// Check for timeout errors
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	
	// Check for context timeout
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	
	return false
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.IsRetryable()
	}
	
	// Check for common retryable errors
	if IsNetworkError(err) || IsTimeoutError(err) {
		return true
	}
	
	return false
}

// WrapError wraps an existing error with additional context
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	
	if appErr, ok := err.(*AppError); ok {
		return &AppError{
			Type:      appErr.Type,
			Message:   fmt.Sprintf("%s: %s", message, appErr.Message),
			Cause:     appErr.Cause,
			Context:   appErr.Context,
			Timestamp: time.Now(),
			Retryable: appErr.Retryable,
		}
	}
	
	// Determine error type from the original error
	errType := UnknownError
	if IsNetworkError(err) {
		errType = NetworkError
	} else if IsTimeoutError(err) {
		errType = TimeoutError
	} else if strings.Contains(err.Error(), "parse") {
		errType = ParseError
	}
	
	return NewError(errType, message, err)
}

// LogError logs an error with appropriate level and context
func LogError(logger *Logger, err error, context map[string]interface{}) {
	if err == nil {
		return
	}
	
	logger = getLoggerOrDefault(logger)
	
	if appErr, ok := err.(*AppError); ok {
		// Merge contexts
		allContext := make(map[string]interface{})
		for k, v := range appErr.Context {
			allContext[k] = v
		}
		for k, v := range context {
			allContext[k] = v
		}
		
		// Create field logger with context
		fieldLogger := logger.WithFields(convertToStringMap(allContext))
		
		// Log based on error type
		switch appErr.Type {
		case NetworkError, TimeoutError:
			fieldLogger.Warn(appErr.Error())
		case HTTPError:
			if statusCode, ok := appErr.Context["status_code"].(int); ok && statusCode >= 500 {
				fieldLogger.Error(appErr.Error())
			} else {
				fieldLogger.Warn(appErr.Error())
			}
		case ConfigError, ValidationError:
			fieldLogger.Error(appErr.Error())
		default:
			fieldLogger.Error(appErr.Error())
		}
	} else {
		// Log regular errors
		if len(context) > 0 {
			fieldLogger := logger.WithFields(convertToStringMap(context))
			fieldLogger.Error(err.Error())
		} else {
			logger.Error(err.Error())
		}
	}
}

// getLoggerOrDefault returns the provided logger or the default logger
func getLoggerOrDefault(logger *Logger) *Logger {
	if logger != nil {
		return logger
	}
	return defaultLogger
}

// convertToStringMap converts a map[string]interface{} to map[string]string
func convertToStringMap(input map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range input {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

// ErrorHandler is a function type for handling errors
type ErrorHandler func(error) error

// DefaultErrorHandler is the default error handler that just logs the error
func DefaultErrorHandler(logger *Logger) ErrorHandler {
	return func(err error) error {
		LogError(logger, err, nil)
		return err
	}
}

// RecoverErrorHandler creates an error handler that recovers from panics
func RecoverErrorHandler(logger *Logger) ErrorHandler {
	return func(err error) error {
		if r := recover(); r != nil {
			panicErr := fmt.Errorf("panic recovered: %v", r)
			LogError(logger, panicErr, map[string]interface{}{
				"original_error": err,
				"panic_value":    r,
			})
			return panicErr
		}
		return err
	}
}
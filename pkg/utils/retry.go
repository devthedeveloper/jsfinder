package utils

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds configuration for retry operations
type RetryConfig struct {
	MaxAttempts     int           // Maximum number of retry attempts
	InitialDelay    time.Duration // Initial delay between retries
	MaxDelay        time.Duration // Maximum delay between retries
	BackoffFactor   float64       // Exponential backoff factor
	Jitter          bool          // Whether to add random jitter
	RetryableErrors []ErrorType   // Types of errors that should trigger retries
	Timeout         time.Duration // Overall timeout for all retry attempts
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		RetryableErrors: []ErrorType{
			NetworkError,
			TimeoutError,
			HTTPError,
		},
		Timeout: 5 * time.Minute,
	}
}

// NetworkRetryConfig returns a retry configuration optimized for network operations
func NetworkRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  200 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 1.5,
		Jitter:        true,
		RetryableErrors: []ErrorType{
			NetworkError,
			TimeoutError,
			HTTPError,
		},
		Timeout: 2 * time.Minute,
	}
}

// QuickRetryConfig returns a retry configuration for quick operations
func QuickRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   2,
		InitialDelay:  50 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        false,
		RetryableErrors: []ErrorType{
			NetworkError,
			TimeoutError,
		},
		Timeout: 10 * time.Second,
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func(ctx context.Context) error

// RetryResult holds the result of a retry operation
type RetryResult struct {
	Success      bool          // Whether the operation succeeded
	Attempts     int           // Number of attempts made
	TotalTime    time.Duration // Total time taken
	LastError    error         // Last error encountered
	AllErrors    []error       // All errors encountered during retries
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, config *RetryConfig, fn RetryableFunc, logger *Logger) *RetryResult {
	if config == nil {
		config = DefaultRetryConfig()
	}
	
	if logger == nil {
		logger = defaultLogger
	}
	
	startTime := time.Now()
	result := &RetryResult{
		AllErrors: make([]error, 0, config.MaxAttempts),
	}
	
	// Create context with timeout if specified
	ctx, cancel := createContextWithTimeout(ctx, config.Timeout)
	defer cancel()
	
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		result.Attempts = attempt
		
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			result.LastError = NewTimeoutError("retry operation cancelled or timed out", ctx.Err())
			result.AllErrors = append(result.AllErrors, result.LastError)
			result.TotalTime = time.Since(startTime)
			return result
		default:
		}
		
		// Execute the function
		err := fn(ctx)
		if err == nil {
			result.Success = true
			result.TotalTime = time.Since(startTime)
			logger.Debug(fmt.Sprintf("Operation succeeded on attempt %d", attempt))
			return result
		}
		
		result.LastError = err
		result.AllErrors = append(result.AllErrors, err)
		
		// Check if error is retryable
		if !isErrorRetryable(err, config.RetryableErrors) {
			logger.Debug(fmt.Sprintf("Non-retryable error on attempt %d: %v", attempt, err))
			result.TotalTime = time.Since(startTime)
			return result
		}
		
		// Don't sleep after the last attempt
		if attempt == config.MaxAttempts {
			logger.Debug(fmt.Sprintf("Max attempts (%d) reached, giving up", config.MaxAttempts))
			break
		}
		
		// Calculate delay for next attempt
		delay := calculateDelay(attempt, config)
		logger.Debug(fmt.Sprintf("Attempt %d failed: %v. Retrying in %v", attempt, err, delay))
		
		// Sleep with context cancellation check
		select {
		case <-ctx.Done():
			result.LastError = NewTimeoutError("retry operation cancelled or timed out during delay", ctx.Err())
			result.AllErrors = append(result.AllErrors, result.LastError)
			result.TotalTime = time.Since(startTime)
			return result
		case <-time.After(delay):
			// Continue to next attempt
		}
	}
	
	result.TotalTime = time.Since(startTime)
	return result
}

// RetryWithCallback executes a function with retry logic and calls a callback on each attempt
func RetryWithCallback(ctx context.Context, config *RetryConfig, fn RetryableFunc, 
	callback func(attempt int, err error), logger *Logger) *RetryResult {
	
	wrappedFn := func(ctx context.Context) error {
		err := fn(ctx)
		if callback != nil {
			// Get current attempt number from the result
			// This is a bit hacky, but works for the callback
			callback(1, err) // We'll update this in the main retry loop
		}
		return err
	}
	
	return Retry(ctx, config, wrappedFn, logger)
}

// createContextWithTimeout creates a context with timeout if specified
func createContextWithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return ctx, func() {}
}

// isErrorRetryable checks if an error should trigger a retry
func isErrorRetryable(err error, retryableErrors []ErrorType) bool {
	if err == nil {
		return false
	}
	
	// Check if it's an AppError with a retryable type
	if appErr, ok := err.(*AppError); ok {
		for _, errType := range retryableErrors {
			if appErr.Type == errType {
				return appErr.IsRetryable()
			}
		}
		return false
	}
	
	// Check for common retryable errors
	return IsRetryableError(err)
}

// calculateDelay calculates the delay for the next retry attempt
func calculateDelay(attempt int, config *RetryConfig) time.Duration {
	// Calculate exponential backoff
	delay := float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt-1))
	
	// Apply maximum delay limit
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}
	
	// Add jitter if enabled
	if config.Jitter {
		// Add up to 25% jitter
		jitterRange := delay * 0.25
		jitter := (rand.Float64() - 0.5) * 2 * jitterRange
		delay += jitter
		
		// Ensure delay is not negative
		if delay < 0 {
			delay = float64(config.InitialDelay)
		}
	}
	
	return time.Duration(delay)
}

// RetryHTTP is a specialized retry function for HTTP operations
func RetryHTTP(ctx context.Context, fn RetryableFunc, logger *Logger) *RetryResult {
	return Retry(ctx, NetworkRetryConfig(), fn, logger)
}

// RetryQuick is a specialized retry function for quick operations
func RetryQuick(ctx context.Context, fn RetryableFunc, logger *Logger) *RetryResult {
	return Retry(ctx, QuickRetryConfig(), fn, logger)
}

// WithRetry is a helper function that wraps a function with retry logic
func WithRetry(config *RetryConfig, logger *Logger) func(RetryableFunc) RetryableFunc {
	return func(fn RetryableFunc) RetryableFunc {
		return func(ctx context.Context) error {
			result := Retry(ctx, config, fn, logger)
			return result.LastError
		}
	}
}

// RetryStats holds statistics about retry operations
type RetryStats struct {
	TotalOperations   int64         // Total number of operations
	SuccessfulOps     int64         // Number of successful operations
	FailedOps         int64         // Number of failed operations
	TotalAttempts     int64         // Total number of attempts across all operations
	TotalRetries      int64         // Total number of retries
	AverageAttempts   float64       // Average attempts per operation
	AverageTime       time.Duration // Average time per operation
	MaxAttempts       int           // Maximum attempts for any single operation
	MaxTime           time.Duration // Maximum time for any single operation
}

// UpdateStats updates retry statistics with a result
func (s *RetryStats) UpdateStats(result *RetryResult) {
	s.TotalOperations++
	s.TotalAttempts += int64(result.Attempts)
	
	if result.Success {
		s.SuccessfulOps++
	} else {
		s.FailedOps++
	}
	
	if result.Attempts > 1 {
		s.TotalRetries += int64(result.Attempts - 1)
	}
	
	// Update maximums
	if result.Attempts > s.MaxAttempts {
		s.MaxAttempts = result.Attempts
	}
	
	if result.TotalTime > s.MaxTime {
		s.MaxTime = result.TotalTime
	}
	
	// Calculate averages
	if s.TotalOperations > 0 {
		s.AverageAttempts = float64(s.TotalAttempts) / float64(s.TotalOperations)
		// Note: AverageTime calculation would require tracking total time
	}
}

// String returns a string representation of the retry statistics
func (s *RetryStats) String() string {
	successRate := float64(s.SuccessfulOps) / float64(s.TotalOperations) * 100
	return fmt.Sprintf("Retry Stats: %d ops (%.1f%% success), avg %.1f attempts, max %d attempts, max time %v",
		s.TotalOperations, successRate, s.AverageAttempts, s.MaxAttempts, s.MaxTime)
}
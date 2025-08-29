package utils

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TimeoutConfig holds configuration for timeout operations
type TimeoutConfig struct {
	OperationTimeout time.Duration // Timeout for individual operations
	GlobalTimeout    time.Duration // Global timeout for all operations
	HeartbeatInterval time.Duration // Interval for heartbeat checks
	GracePeriod      time.Duration // Grace period before force termination
}

// DefaultTimeoutConfig returns a default timeout configuration
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		OperationTimeout:  30 * time.Second,
		GlobalTimeout:     5 * time.Minute,
		HeartbeatInterval: 5 * time.Second,
		GracePeriod:       10 * time.Second,
	}
}

// HTTPTimeoutConfig returns a timeout configuration optimized for HTTP operations
func HTTPTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		OperationTimeout:  15 * time.Second,
		GlobalTimeout:     2 * time.Minute,
		HeartbeatInterval: 3 * time.Second,
		GracePeriod:       5 * time.Second,
	}
}

// CrawlerTimeoutConfig returns a timeout configuration optimized for web crawling
func CrawlerTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		OperationTimeout:  20 * time.Second,
		GlobalTimeout:     10 * time.Minute,
		HeartbeatInterval: 10 * time.Second,
		GracePeriod:       15 * time.Second,
	}
}

// TimeoutManager manages timeouts for operations
type TimeoutManager struct {
	config    *TimeoutConfig
	logger    *Logger
	operations map[string]*OperationContext
	mutex     sync.RWMutex
	globalCtx context.Context
	cancel    context.CancelFunc
	startTime time.Time
}

// OperationContext holds context for a single operation
type OperationContext struct {
	ID        string
	Ctx       context.Context
	Cancel    context.CancelFunc
	StartTime time.Time
	Timeout   time.Duration
	Heartbeat chan struct{}
	Done      chan struct{}
}

// NewTimeoutManager creates a new timeout manager
func NewTimeoutManager(config *TimeoutConfig, logger *Logger) *TimeoutManager {
	if config == nil {
		config = DefaultTimeoutConfig()
	}
	
	if logger == nil {
		logger = defaultLogger
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), config.GlobalTimeout)
	
	tm := &TimeoutManager{
		config:     config,
		logger:     logger,
		operations: make(map[string]*OperationContext),
		globalCtx:  ctx,
		cancel:     cancel,
		startTime:  time.Now(),
	}
	
	// Start global timeout monitor
	go tm.monitorGlobalTimeout()
	
	return tm
}

// CreateOperation creates a new operation with timeout
func (tm *TimeoutManager) CreateOperation(id string, timeout time.Duration) *OperationContext {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	// Use operation timeout if not specified
	if timeout == 0 {
		timeout = tm.config.OperationTimeout
	}
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(tm.globalCtx, timeout)
	
	opCtx := &OperationContext{
		ID:        id,
		Ctx:       ctx,
		Cancel:    cancel,
		StartTime: time.Now(),
		Timeout:   timeout,
		Heartbeat: make(chan struct{}, 1),
		Done:      make(chan struct{}),
	}
	
	tm.operations[id] = opCtx
	
	// Start operation monitor
	go tm.monitorOperation(opCtx)
	
	tm.logger.Debug(fmt.Sprintf("Created operation %s with timeout %v", id, timeout))
	return opCtx
}

// CompleteOperation marks an operation as completed
func (tm *TimeoutManager) CompleteOperation(id string) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	if opCtx, exists := tm.operations[id]; exists {
		close(opCtx.Done)
		opCtx.Cancel()
		delete(tm.operations, id)
		
		duration := time.Since(opCtx.StartTime)
		tm.logger.Debug(fmt.Sprintf("Completed operation %s in %v", id, duration))
	}
}

// CancelOperation cancels a specific operation
func (tm *TimeoutManager) CancelOperation(id string) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	if opCtx, exists := tm.operations[id]; exists {
		opCtx.Cancel()
		close(opCtx.Done)
		delete(tm.operations, id)
		
		tm.logger.Warn(fmt.Sprintf("Cancelled operation %s", id))
	}
}

// SendHeartbeat sends a heartbeat for an operation
func (tm *TimeoutManager) SendHeartbeat(id string) {
	tm.mutex.RLock()
	opCtx, exists := tm.operations[id]
	tm.mutex.RUnlock()
	
	if exists {
		select {
		case opCtx.Heartbeat <- struct{}{}:
			// Heartbeat sent
		default:
			// Channel full, skip
		}
	}
}

// GetOperationContext returns the context for an operation
func (tm *TimeoutManager) GetOperationContext(id string) (context.Context, bool) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	
	if opCtx, exists := tm.operations[id]; exists {
		return opCtx.Ctx, true
	}
	return nil, false
}

// GetActiveOperations returns the number of active operations
func (tm *TimeoutManager) GetActiveOperations() int {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return len(tm.operations)
}

// Shutdown gracefully shuts down the timeout manager
func (tm *TimeoutManager) Shutdown() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	// Cancel all operations
	for id, opCtx := range tm.operations {
		opCtx.Cancel()
		close(opCtx.Done)
		tm.logger.Debug(fmt.Sprintf("Shutdown operation %s", id))
	}
	
	// Clear operations
	tm.operations = make(map[string]*OperationContext)
	
	// Cancel global context
	tm.cancel()
	
	tm.logger.Info("Timeout manager shutdown completed")
}

// monitorGlobalTimeout monitors the global timeout
func (tm *TimeoutManager) monitorGlobalTimeout() {
	<-tm.globalCtx.Done()
	
	if tm.globalCtx.Err() == context.DeadlineExceeded {
		tm.logger.Error(fmt.Sprintf("Global timeout exceeded after %v", tm.config.GlobalTimeout))
		
		// Cancel all operations
		tm.mutex.RLock()
		operations := make([]*OperationContext, 0, len(tm.operations))
		for _, opCtx := range tm.operations {
			operations = append(operations, opCtx)
		}
		tm.mutex.RUnlock()
		
		for _, opCtx := range operations {
			opCtx.Cancel()
		}
	}
}

// monitorOperation monitors a single operation
func (tm *TimeoutManager) monitorOperation(opCtx *OperationContext) {
	heartbeatTicker := time.NewTicker(tm.config.HeartbeatInterval)
	defer heartbeatTicker.Stop()
	
	lastHeartbeat := time.Now()
	
	for {
		select {
		case <-opCtx.Done:
			// Operation completed normally
			return
		
		case <-opCtx.Ctx.Done():
			// Operation timed out or was cancelled
			if opCtx.Ctx.Err() == context.DeadlineExceeded {
				duration := time.Since(opCtx.StartTime)
				tm.logger.Warn(fmt.Sprintf("Operation %s timed out after %v (timeout: %v)", 
					opCtx.ID, duration, opCtx.Timeout))
			}
			tm.CancelOperation(opCtx.ID)
			return
		
		case <-opCtx.Heartbeat:
			// Received heartbeat
			lastHeartbeat = time.Now()
			tm.logger.Debug(fmt.Sprintf("Received heartbeat for operation %s", opCtx.ID))
		
		case <-heartbeatTicker.C:
			// Check for heartbeat timeout
			if time.Since(lastHeartbeat) > tm.config.HeartbeatInterval*2 {
				tm.logger.Warn(fmt.Sprintf("No heartbeat received for operation %s in %v", 
					opCtx.ID, time.Since(lastHeartbeat)))
			}
		}
	}
}

// WithTimeout executes a function with a timeout
func WithTimeout(timeout time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	done := make(chan error, 1)
	
	go func() {
		done <- fn(ctx)
	}()
	
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return NewTimeoutError(fmt.Sprintf("operation timed out after %v", timeout), ctx.Err())
	}
}

// WithDeadline executes a function with a deadline
func WithDeadline(deadline time.Time, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	
	done := make(chan error, 1)
	
	go func() {
		done <- fn(ctx)
	}()
	
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return NewTimeoutError(fmt.Sprintf("operation exceeded deadline %v", deadline), ctx.Err())
	}
}

// TimeoutWrapper wraps a function with timeout handling
type TimeoutWrapper struct {
	timeout time.Duration
	logger  *Logger
}

// NewTimeoutWrapper creates a new timeout wrapper
func NewTimeoutWrapper(timeout time.Duration, logger *Logger) *TimeoutWrapper {
	return &TimeoutWrapper{
		timeout: timeout,
		logger:  logger,
	}
}

// Wrap wraps a function with timeout handling
func (tw *TimeoutWrapper) Wrap(fn func(ctx context.Context) error) func() error {
	return func() error {
		start := time.Now()
		err := WithTimeout(tw.timeout, fn)
		duration := time.Since(start)
		
		if err != nil {
			if IsTimeoutError(err) {
				tw.logger.Warn(fmt.Sprintf("Function timed out after %v (timeout: %v)", duration, tw.timeout))
			} else {
				tw.logger.Error(fmt.Sprintf("Function failed after %v: %v", duration, err))
			}
		} else {
			tw.logger.Debug(fmt.Sprintf("Function completed in %v", duration))
		}
		
		return err
	}
}

// BatchTimeout handles timeouts for batch operations
type BatchTimeout struct {
	operationTimeout time.Duration
	batchTimeout     time.Duration
	maxConcurrency   int
	logger           *Logger
}

// NewBatchTimeout creates a new batch timeout handler
func NewBatchTimeout(operationTimeout, batchTimeout time.Duration, maxConcurrency int, logger *Logger) *BatchTimeout {
	return &BatchTimeout{
		operationTimeout: operationTimeout,
		batchTimeout:     batchTimeout,
		maxConcurrency:   maxConcurrency,
		logger:           logger,
	}
}

// ExecuteBatch executes a batch of operations with timeout handling
func (bt *BatchTimeout) ExecuteBatch(ctx context.Context, operations []func(ctx context.Context) error) []error {
	// Create context with batch timeout
	batchCtx, cancel := context.WithTimeout(ctx, bt.batchTimeout)
	defer cancel()
	
	results := make([]error, len(operations))
	semaphore := make(chan struct{}, bt.maxConcurrency)
	var wg sync.WaitGroup
	
	for i, op := range operations {
		wg.Add(1)
		go func(index int, operation func(ctx context.Context) error) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// Execute operation with timeout
			opCtx, opCancel := context.WithTimeout(batchCtx, bt.operationTimeout)
			defer opCancel()
			
			results[index] = operation(opCtx)
		}(i, op)
	}
	
	// Wait for all operations to complete or batch timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		bt.logger.Debug(fmt.Sprintf("Batch completed with %d operations", len(operations)))
	case <-batchCtx.Done():
		bt.logger.Warn(fmt.Sprintf("Batch timed out after %v", bt.batchTimeout))
		// Fill remaining results with timeout errors
		for i, result := range results {
			if result == nil {
				results[i] = NewTimeoutError("operation timed out in batch", batchCtx.Err())
			}
		}
	}
	
	return results
}
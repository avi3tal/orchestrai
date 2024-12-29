package types

import (
	"context"
	"fmt"
	"time"
)

// StateTransformer represents a function that transforms state
type StateTransformer func(ctx context.Context, state State) (State, error)

// StateValidator represents a function that validates state
type StateValidator func(state State) error

// NodeMiddleware represents a function that wraps a StateTransformer
type NodeMiddleware func(StateTransformer) StateTransformer

// RetryPolicy defines how to handle retries for failed operations
type RetryPolicy struct {
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int `json:"max_attempts"`

	// InitialDelay is the initial delay between retries in milliseconds
	InitialDelay int64 `json:"initial_delay_ms"`

	// MaxDelay is the maximum delay between retries in milliseconds
	MaxDelay int64 `json:"max_delay_ms"`

	// BackoffMultiplier is the multiplier to use for exponential backoff
	BackoffMultiplier float64 `json:"backoff_multiplier"`

	// RetryableErrors is a list of error types that should trigger retries
	RetryableErrors []string `json:"retryable_errors,omitempty"`

	// MaxDuration is the maximum total duration for all retries
	MaxDuration time.Duration `json:"max_duration"`

	// RetryIf is a custom function to determine if an error should trigger a retry
	RetryIf func(error) bool `json:"-"`
}

// NewRetryPolicy creates a default retry policy
func NewRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:       3,
		InitialDelay:      100,  // 100ms
		MaxDelay:          5000, // 5s
		BackoffMultiplier: 2.0,
		MaxDuration:       time.Minute,
	}
}

// WithMaxAttempts sets the maximum number of retry attempts
func (p *RetryPolicy) WithMaxAttempts(attempts int) *RetryPolicy {
	p.MaxAttempts = attempts
	return p
}

// WithInitialDelay sets the initial delay in milliseconds
func (p *RetryPolicy) WithInitialDelay(delay int64) *RetryPolicy {
	p.InitialDelay = delay
	return p
}

// WithMaxDelay sets the maximum delay in milliseconds
func (p *RetryPolicy) WithMaxDelay(delay int64) *RetryPolicy {
	p.MaxDelay = delay
	return p
}

// WithBackoffMultiplier sets the backoff multiplier
func (p *RetryPolicy) WithBackoffMultiplier(multiplier float64) *RetryPolicy {
	p.BackoffMultiplier = multiplier
	return p
}

// WithMaxDuration sets the maximum total duration for retries
func (p *RetryPolicy) WithMaxDuration(duration time.Duration) *RetryPolicy {
	p.MaxDuration = duration
	return p
}

// WithRetryableErrors sets the list of retryable error types
func (p *RetryPolicy) WithRetryableErrors(errors []string) *RetryPolicy {
	p.RetryableErrors = errors
	return p
}

// WithRetryIf sets a custom retry condition
func (p *RetryPolicy) WithRetryIf(condition func(error) bool) *RetryPolicy {
	p.RetryIf = condition
	return p
}

// ShouldRetry determines if a retry should be attempted based on the error
func (p *RetryPolicy) ShouldRetry(err error) bool {
	if p.RetryIf != nil {
		return p.RetryIf(err)
	}

	if len(p.RetryableErrors) == 0 {
		return true // Retry all errors if no specific types are specified
	}

	errStr := err.Error()
	for _, retryableErr := range p.RetryableErrors {
		if retryableErr == errStr {
			return true
		}
	}

	return false
}

// DefaultMiddlewares provides commonly used node middleware
var DefaultMiddlewares = struct {
	// Logger logs state transformations
	//Logger func(name string) NodeMiddleware

	// Metrics records metrics about state transformations
	//Metrics func(name string) NodeMiddleware

	// Timeout adds a timeout to state transformations
	Timeout func(timeout time.Duration) NodeMiddleware

	// Validator adds state validation before and after transformation
	Validator func(validator StateValidator) NodeMiddleware
}{
	//Logger: func(name string) NodeMiddleware {
	//	return func(next StateTransformer) StateTransformer {
	//		return func(ctx context.Context, state State) (State, error) {
	//			// Log before transformation
	//			log.Printf("[%s] Transforming state: %v", name, state)
	//
	//			newState, err := next(ctx, state)
	//
	//			// Log after transformation
	//			if err != nil {
	//				log.Printf("[%s] Transformation failed: %v", name, err)
	//			} else {
	//				log.Printf("[%s] State transformed: %v", name, newState)
	//			}
	//
	//			return newState, err
	//		}
	//	}
	//},
	//
	//Metrics: func(name string) NodeMiddleware {
	//	return func(next StateTransformer) StateTransformer {
	//		return func(ctx context.Context, state State) (State, error) {
	//			start := time.Now()
	//
	//			newState, err := next(ctx, state)
	//
	//			duration := time.Since(start)
	//			// Record metrics here (using your metrics system)
	//			metrics.RecordDuration(name, duration)
	//			if err != nil {
	//				metrics.RecordError(name)
	//			}
	//
	//			return newState, err
	//		}
	//	}
	//},

	Timeout: func(timeout time.Duration) NodeMiddleware {
		return func(next StateTransformer) StateTransformer {
			return func(ctx context.Context, state State) (State, error) {
				ctx, cancel := context.WithTimeout(ctx, timeout)
				defer cancel()

				return next(ctx, state)
			}
		}
	},

	Validator: func(validator StateValidator) NodeMiddleware {
		return func(next StateTransformer) StateTransformer {
			return func(ctx context.Context, state State) (State, error) {
				// Validate input state
				if err := validator(state); err != nil {
					return nil, fmt.Errorf("input state validation failed: %w", err)
				}

				newState, err := next(ctx, state)
				if err != nil {
					return nil, err
				}

				// Validate output state
				if err := validator(newState); err != nil {
					return nil, fmt.Errorf("output state validation failed: %w", err)
				}

				return newState, nil
			}
		}
	},
}

// NodeExecutor represents a node that can execute operations on state
type NodeExecutor interface {
	// Execute processes the input state and returns a new state
	Execute(ctx context.Context, state State) (State, error)

	// GetID returns the node's unique identifier
	GetID() string

	// GetType returns the node's type
	GetType() string
}

// ExecutionResult represents the result of a node execution
type ExecutionResult struct {
	// State is the resulting state after execution
	State State

	// NextNode is the ID of the next node to execute (if any)
	NextNode string

	// Metadata contains execution-specific information
	Metadata map[string]interface{}
}

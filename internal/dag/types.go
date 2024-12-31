package dag

import (
	"context"
)

// State represents the base interface for any state type
type State interface {
	// Validate validates the state
	Validate() error
}

// Channel represents state management operations
type Channel[T State] interface {
	// Read reads the current state from the channel
	Read(ctx context.Context, config Config[T]) (T, error)
	// Write writes a new state to the channel
	Write(ctx context.Context, value T, config Config[T]) error
}

// Config represents runtime configuration for graph execution
type Config[T State] struct {
	ThreadID     string          // Unique identifier for this execution thread
	MaxSteps     int             // Maximum number of steps to execute
	Timeout      int             // Timeout in seconds
	Checkpointer Checkpointer[T] // Optional checkpointer for state persistence
	Configurable map[string]any  // Additional configuration parameters
	Debug        bool            // Enable execution tracing
}

// Checkpointer handles state persistence with generic type
type Checkpointer[T State] interface {
	// Save persists the current state
	Save(ctx context.Context, config Config[T], state T) error
	// Load retrieves a previously saved state
	Load(ctx context.Context, config Config[T]) (T, error)
}

// NodeSpec represents a node's specification
type NodeSpec[T State] struct {
	Name        string
	Function    func(context.Context, T, Config[T]) (T, error)
	Metadata    map[string]any
	RetryPolicy *RetryPolicy
}

// RetryPolicy defines how a node should handle failures
type RetryPolicy struct {
	MaxAttempts int
	Delay       int // delay in seconds between attempts
}

// Edge represents a connection between nodes
type Edge struct {
	From     string
	To       string
	Metadata map[string]any
}

// Branch represents a conditional branch in the graph
type Branch[T State] struct {
	Path     func(context.Context, T, Config[T]) string
	Then     string
	Metadata map[string]any
}

// Constants for special nodes
const (
	START = "START"
	END   = "END"
)

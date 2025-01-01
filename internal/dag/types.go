package dag

import (
	"context"
)

type Mergeable[T any] interface {
	Merge(T) T
}

// State represents the base interface for any state type
type State interface {
	// Validate validates the state
	Validate() error
}

// GraphState Combine both interfaces for graph states
type GraphState[T any] interface {
	State
	Mergeable[T]
}

// Channel represents state management operations
type Channel[T GraphState[T]] interface {
	// Read reads the current state from the channel
	Read(ctx context.Context, config Config[T]) (T, error)
	// Write writes a new state to the channel
	Write(ctx context.Context, value T, config Config[T]) error
}

// Checkpointer handles state persistence with generic type
type Checkpointer[T GraphState[T]] interface {
	// Save persists the current state
	Save(ctx context.Context, config Config[T], data *CheckpointData[T]) error
	// Load retrieves a previously saved state
	Load(ctx context.Context, config Config[T]) (*CheckpointData[T], error)
}

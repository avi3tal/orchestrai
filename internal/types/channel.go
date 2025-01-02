package types

import (
	"context"
	"github.com/avi3tal/orchestrai/internal/state"
)

// Channel represents state management operations
type Channel[T state.GraphState[T]] interface {
	// Read reads the current state from the channel
	Read(ctx context.Context, config Config[T]) (T, error)
	// Write writes a new state to the channel
	Write(ctx context.Context, value T, config Config[T]) error
}

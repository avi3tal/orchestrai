package types

import (
	"context"

	"github.com/avi3tal/orchestrai/internal/state"
)

// Channel represents state management operations.
type Channel interface {
	// Read reads the current state from the channel
	Read(ctx context.Context, config Config) (any, error)
	// Write writes a new state to the channel
	Write(ctx context.Context, value state.GraphState, config Config) error
}

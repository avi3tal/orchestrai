package types

import (
	"context"
	"github.com/avi3tal/orchestrai/internal/state"
	"time"
)

type CheckpointKey struct {
	GraphID  string
	ThreadID string
}

type CheckpointMeta struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	Steps     int
	Status    NodeExecutionStatus
	NodeQueue []string
}

type Checkpoint[T state.GraphState[T]] struct {
	Key    CheckpointKey
	Meta   CheckpointMeta
	State  T
	NodeID string
}

// DataPoint Extended checkpoint data
type DataPoint[T state.GraphState[T]] struct {
	State       T
	Status      NodeExecutionStatus
	CurrentNode string
	Steps       int
	NodeQueue   []string
}

// Checkpointer handles state persistence with generic type
type Checkpointer[T state.GraphState[T]] interface {
	// Save persists the current state
	Save(ctx context.Context, config Config[T], data *DataPoint[T]) error
	// Load retrieves a previously saved state
	Load(ctx context.Context, config Config[T]) (*DataPoint[T], error)
}

// CheckpointStore interface defines persistent storage operations
type CheckpointStore[T state.GraphState[T]] interface {
	Save(ctx context.Context, checkpoint Checkpoint[T]) error
	Load(ctx context.Context, key CheckpointKey) (*Checkpoint[T], error)
	Delete(ctx context.Context, key CheckpointKey) error
}

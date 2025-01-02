package types

import (
	"context"
	"time"

	"github.com/avi3tal/orchestrai/internal/state"
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

type Checkpoint struct {
	Key    CheckpointKey
	Meta   CheckpointMeta
	State  state.GraphState
	NodeID string
}

// DataPoint Extended checkpoint data
type DataPoint struct {
	State       state.GraphState
	Status      NodeExecutionStatus
	CurrentNode string
	Steps       int
	NodeQueue   []string
}

// Checkpointer handles state persistence with generic type
type Checkpointer interface {
	// Save persists the current state
	Save(ctx context.Context, config Config, data *DataPoint) error
	// Load retrieves a previously saved state
	Load(ctx context.Context, config Config) (*DataPoint, error)
}

// CheckpointStore interface defines persistent storage operations
type CheckpointStore interface {
	Save(ctx context.Context, checkpoint Checkpoint) error
	Load(ctx context.Context, key CheckpointKey) (*Checkpoint, error)
	Delete(ctx context.Context, key CheckpointKey) error
}

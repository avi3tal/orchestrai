package types

import (
	"context"
	"time"
)

// StateID uniquely identifies a state instance
type StateID string

// CheckpointID uniquely identifies a checkpoint
type CheckpointID string

// Checkpoint represents a recoverable point in the workflow
type Checkpoint struct {
	// ID uniquely identifies this checkpoint
	ID CheckpointID `json:"id"`

	// GraphID identifies the graph instance
	GraphID string `json:"graph_id"`

	// NodeID identifies the node that created this checkpoint
	NodeID string `json:"node_id"`

	// State is the current state at this checkpoint
	State State `json:"state"`

	// Metadata contains checkpoint information
	Metadata CheckpointMetadata `json:"metadata"`

	// ParentID references the parent checkpoint if any
	ParentID CheckpointID `json:"parent_id,omitempty"`
}

// CheckpointMetadata holds information about the checkpoint
type CheckpointMetadata struct {
	// CreatedAt is when the checkpoint was created
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the checkpoint expires
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// Step is the execution step number
	Step int64 `json:"step"`

	// Tags are optional labels for organizing checkpoints
	Tags []string `json:"tags,omitempty"`

	// Custom metadata
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// Checkpointer manages graph execution checkpoints
type Checkpointer interface {
	// Save creates a new checkpoint
	Save(ctx context.Context, checkpoint *Checkpoint) error

	// Load retrieves a checkpoint by ID
	Load(ctx context.Context, id CheckpointID) (*Checkpoint, error)

	// List returns checkpoints matching the filter
	List(ctx context.Context, filter CheckpointFilter) ([]*Checkpoint, error)

	// Delete removes a checkpoint
	Delete(ctx context.Context, id CheckpointID) error
}

// Store provides persistent storage for state and checkpoint data
type Store interface {
	// Put stores a value with namespace and key
	Put(ctx context.Context, namespace string, key string, value interface{}) error

	// Get retrieves a value by namespace and key
	Get(ctx context.Context, namespace string, key string) (interface{}, error)

	// List returns all keys in a namespace
	List(ctx context.Context, namespace string) ([]string, error)

	// Delete removes a value
	Delete(ctx context.Context, namespace string, key string) error

	// Watch monitors changes in a namespace
	Watch(ctx context.Context, namespace string) (<-chan StoreEvent, error)
}

// StoreEvent represents a change in the store
type StoreEvent struct {
	// Type is the event type (put, delete, etc)
	Type string `json:"type"`

	// Namespace is the affected namespace
	Namespace string `json:"namespace"`

	// Key is the affected key
	Key string `json:"key"`

	// Value is the new value if applicable
	Value interface{} `json:"value,omitempty"`

	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`
}

// CheckpointFilter defines criteria for listing checkpoints
type CheckpointFilter struct {
	// GraphID filters by graph instance
	GraphID string `json:"graph_id,omitempty"`

	// NodeID filters by node
	NodeID string `json:"node_id,omitempty"`

	// FromStep filters by minimum step number
	FromStep *int64 `json:"from_step,omitempty"`

	// ToStep filters by maximum step number
	ToStep *int64 `json:"to_step,omitempty"`

	// Tags filters by tags (AND condition)
	Tags []string `json:"tags,omitempty"`

	// CreatedAfter filters by creation time
	CreatedAfter *time.Time `json:"created_after,omitempty"`

	// CreatedBefore filters by creation time
	CreatedBefore *time.Time `json:"created_before,omitempty"`
}

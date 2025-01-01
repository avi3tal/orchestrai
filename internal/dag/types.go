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

// CheckpointStore interface defines persistent storage operations
type CheckpointStore[T GraphState[T]] interface {
	Save(ctx context.Context, checkpoint Checkpoint[T]) error
	Load(ctx context.Context, key CheckpointKey) (*Checkpoint[T], error)
	List(ctx context.Context, graphID string) ([]CheckpointKey, error)
	Delete(ctx context.Context, key CheckpointKey) error
}

type Grapher[T GraphState[T]] interface {
	AddNode(name string, fn func(context.Context, T, Config[T]) (NodeResponse[T], error), metadata map[string]any) error
	AddEdge(from, to string, metadata map[string]any) error
	AddBranch(from string, path func(context.Context, T, Config[T]) string, then string, metadata map[string]any) error
	AddChannel(name string, channel Channel[T]) error
	Compile(opt ...CompilationOption[T]) (*CompiledGraph[T], error)
}

type CompiledGrapher[T GraphState[T]] interface {
	Run(ctx context.Context, initialState T, opt ...ExecutionOption[T]) (T, error)
}

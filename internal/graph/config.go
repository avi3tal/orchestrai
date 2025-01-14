package graph

import (
	"github.com/avi3tal/orchestrai/pkg/checkpoints"
	"github.com/avi3tal/orchestrai/pkg/state"
	"github.com/avi3tal/orchestrai/pkg/types"
	"github.com/google/uuid"
)

const (
	defaultMaxSteps = 20
	defaultTimeout  = 60
)

func NewConfig[T state.GraphState[T]](graphID string, opt ...CompilationOption[T]) types.Config[T] {
	opts := types.Config[T]{
		GraphID:  graphID,
		ThreadID: uuid.New().String(), // generate default thread ID
		MaxSteps: defaultMaxSteps,
		Timeout:  defaultTimeout,
	}
	for _, o := range opt {
		o(&opts)
	}
	return opts
}

type CompilationOption[T state.GraphState[T]] func(*types.Config[T])

// WithMaxSteps sets the maximum number of steps to execute
func WithMaxSteps[T state.GraphState[T]](steps int) CompilationOption[T] {
	return func(c *types.Config[T]) {
		c.MaxSteps = steps
	}
}

// WithTimeout sets the execution timeout in seconds
func WithTimeout[T state.GraphState[T]](timeout int) CompilationOption[T] {
	return func(c *types.Config[T]) {
		c.Timeout = timeout
	}
}

// WithCheckpointStore sets the checkpointer for state persistence
func WithCheckpointStore[T state.GraphState[T]](store types.CheckpointStore[T]) CompilationOption[T] {
	return func(c *types.Config[T]) {
		c.Checkpointer = checkpoints.NewStateCheckpointer(store)
	}
}

// WithDebug enables execution tracing
func WithDebug[T state.GraphState[T]]() CompilationOption[T] {
	return func(c *types.Config[T]) {
		c.Debug = true
	}
}

type ExecutionOption[T state.GraphState[T]] func(*types.Config[T])

// WithThreadID sets the unique thread identifier
func WithThreadID[T state.GraphState[T]](id string) ExecutionOption[T] {
	return func(c *types.Config[T]) {
		c.ThreadID = id
	}
}

// WithConfigurable sets additional configuration parameters
func WithConfigurable[T state.GraphState[T]](config map[string]any) ExecutionOption[T] {
	return func(c *types.Config[T]) {
		c.Configurable = config
	}
}

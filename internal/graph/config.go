package graph

import (
	"github.com/avi3tal/orchestrai/internal/checkpoints"
	"github.com/avi3tal/orchestrai/internal/types"
	"github.com/google/uuid"
)

const (
	defaultMaxSteps = 20
	defaultTimeout  = 60
)

func NewConfig(graphID string, opt ...CompilationOption) types.Config {
	opts := types.Config{
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

type CompilationOption func(*types.Config)

// WithMaxSteps sets the maximum number of steps to execute
func WithMaxSteps(steps int) CompilationOption {
	return func(c *types.Config) {
		c.MaxSteps = steps
	}
}

// WithTimeout sets the execution timeout in seconds
func WithTimeout(timeout int) CompilationOption {
	return func(c *types.Config) {
		c.Timeout = timeout
	}
}

// WithCheckpointStore sets the checkpointer for state persistence
func WithCheckpointStore(store types.CheckpointStore) CompilationOption {
	return func(c *types.Config) {
		c.Checkpointer = checkpoints.NewStateCheckpointer(store)
	}
}

// WithDebug enables execution tracing
func WithDebug() CompilationOption {
	return func(c *types.Config) {
		c.Debug = true
	}
}

type ExecutionOption func(*types.Config)

// WithThreadID sets the unique thread identifier
func WithThreadID(id string) ExecutionOption {
	return func(c *types.Config) {
		c.ThreadID = id
	}
}

// WithConfigurable sets additional configuration parameters
func WithConfigurable(config map[string]any) ExecutionOption {
	return func(c *types.Config) {
		c.Configurable = config
	}
}

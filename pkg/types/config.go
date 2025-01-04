package types

import "github.com/avi3tal/orchestrai/pkg/state"

// Config represents runtime configuration for graph execution
type Config[T state.GraphState[T]] struct {
	GraphID      string          // Unique identifier for the graph
	ThreadID     string          // Unique identifier for this execution thread
	MaxSteps     int             // Maximum number of steps to execute
	Timeout      int             // Timeout in seconds
	Checkpointer Checkpointer[T] // Optional checkpointer for state persistence
	Configurable map[string]any  // Additional configuration parameters
	Debug        bool            // Enable execution tracing
}

func (c *Config[T]) Clone() Config[T] {
	return Config[T]{
		GraphID:      c.GraphID,
		ThreadID:     c.ThreadID,
		MaxSteps:     c.MaxSteps,
		Timeout:      c.Timeout,
		Checkpointer: c.Checkpointer,
		Configurable: c.Configurable,
		Debug:        c.Debug,
	}
}

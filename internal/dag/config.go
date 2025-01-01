package dag

import "github.com/google/uuid"

// Config represents runtime configuration for graph execution
type Config[T GraphState[T]] struct {
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

func NewConfig[T GraphState[T]](graphID string, opt ...CompilationOption[T]) Config[T] {
	opts := Config[T]{
		GraphID:  graphID,
		ThreadID: uuid.New().String(), // generate default thread ID
		MaxSteps: 20,
		Timeout:  60,
	}
	for _, o := range opt {
		o(&opts)
	}
	return opts
}

type CompilationOption[T GraphState[T]] func(*Config[T])

// WithMaxSteps sets the maximum number of steps to execute
func WithMaxSteps[T GraphState[T]](steps int) CompilationOption[T] {
	return func(c *Config[T]) {
		c.MaxSteps = steps
	}
}

// WithTimeout sets the execution timeout in seconds
func WithTimeout[T GraphState[T]](timeout int) CompilationOption[T] {
	return func(c *Config[T]) {
		c.Timeout = timeout
	}
}

// WithCheckpointStore sets the checkpointer for state persistence
func WithCheckpointStore[T GraphState[T]](store CheckpointStore[T]) CompilationOption[T] {
	return func(c *Config[T]) {
		c.Checkpointer = NewStateCheckpointer(store)
	}
}

// WithDebug enables execution tracing
func WithDebug[T GraphState[T]]() CompilationOption[T] {
	return func(c *Config[T]) {
		c.Debug = true
	}
}

type ExecutionOption[T GraphState[T]] func(*Config[T])

// WithThreadID sets the unique thread identifier
func WithThreadID[T GraphState[T]](id string) ExecutionOption[T] {
	return func(c *Config[T]) {
		c.ThreadID = id
	}
}

// WithConfigurable sets additional configuration parameters
func WithConfigurable[T GraphState[T]](config map[string]any) ExecutionOption[T] {
	return func(c *Config[T]) {
		c.Configurable = config
	}
}

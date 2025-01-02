package types

// Config represents runtime configuration for graph execution
type Config struct {
	GraphID      string         // Unique identifier for the graph
	ThreadID     string         // Unique identifier for this execution thread
	MaxSteps     int            // Maximum number of steps to execute
	Timeout      int            // Timeout in seconds
	Checkpointer Checkpointer   // Optional checkpointer for state persistence
	Configurable map[string]any // Additional configuration parameters
	Debug        bool           // Enable execution tracing
}

func (c *Config) Clone() Config {
	return Config{
		GraphID:      c.GraphID,
		ThreadID:     c.ThreadID,
		MaxSteps:     c.MaxSteps,
		Timeout:      c.Timeout,
		Checkpointer: c.Checkpointer,
		Configurable: c.Configurable,
		Debug:        c.Debug,
	}
}

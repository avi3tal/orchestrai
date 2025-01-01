package dag

import "context"

// NodeSpec represents a node's specification
type NodeSpec[T GraphState[T]] struct {
	Name        string
	Function    func(context.Context, T, Config[T]) (NodeResponse[T], error)
	Metadata    map[string]any
	RetryPolicy *RetryPolicy
}

// RetryPolicy defines how a node should handle failures
type RetryPolicy struct {
	MaxAttempts int
	Delay       int // delay in seconds between attempts
}

// Edge represents a connection between nodes
type Edge struct {
	From     string
	To       string
	Metadata map[string]any
}

// Branch represents a conditional branch in the graph
type Branch[T GraphState[T]] struct {
	Path     func(context.Context, T, Config[T]) string
	Then     string
	Metadata map[string]any
}

// Constants for special nodes
const (
	START = "START"
	END   = "END"
)

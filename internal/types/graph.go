package types

import "context"

// Constants for special graph nodes
const (
	START = "__start__"
	END   = "__end__"
)

// Validator represents any component that can be validated
type Validator interface {
	// Validate performs validation and returns an error if validation fails
	Validate() error
}

// Node represents a single vertex in the graph
type Node interface {
	// GetID returns the unique identifier of the node
	GetID() string
}

// Edge represents a directed connection between two nodes
type Edge struct {
	// From is the source node ID
	From string

	// To is the target node ID
	To string

	// Condition is the optional condition function that determines if this edge should be followed
	// If nil, edge is always followed
	Condition EdgeCondition
}

// EdgeCondition evaluates whether an edge should be followed based on the current state
// Returns the target node ID ("end" for END node) or error if evaluation fails
type EdgeCondition func(ctx context.Context, state State) (string, error)

// Graph represents a directed graph structure that can be built and modified
type Graph interface {
	// AddNode adds a new node to the graph
	AddNode(node Node) error

	// AddEdge creates an edge between two nodes
	AddEdge(from, to string) error

	// AddConditionalEdge creates an edge with a condition between two nodes
	AddConditionalEdge(from string, condition EdgeCondition, to string) error

	// Validate ensures the graph is properly constructed
	Validate() error

	// Compile creates an immutable compiled version of the graph
	Compile(opts ...CompileOption) (CompiledGraph, error)
}

// CompiledGraph represents an immutable graph that can be executed
type CompiledGraph interface {
	// Execute runs the graph with given input state
	Execute(ctx context.Context, state State) (State, error)

	// GetNode returns a node by ID
	GetNode(id string) (Node, bool)

	// GetEdges returns all edges from a given node
	GetEdges(nodeID string) []Edge
}

// CompileOption configures how the graph is compiled
type CompileOption func(*CompileOptions)

// CompileOptions holds configuration for graph compilation
type CompileOptions struct {
	// Debug enables detailed logging and validation
	Debug bool

	// InterruptBefore defines nodes that require interruption before execution
	InterruptBefore []string

	// InterruptAfter defines nodes that require interruption after execution
	InterruptAfter []string
}

func WithDebug(debug bool) CompileOption {
	return func(o *CompileOptions) {
		o.Debug = debug
	}
}

func WithInterruptBefore(nodes ...string) CompileOption {
	return func(o *CompileOptions) {
		o.InterruptBefore = nodes
	}
}

func WithInterruptAfter(nodes ...string) CompileOption {
	return func(o *CompileOptions) {
		o.InterruptAfter = nodes
	}
}

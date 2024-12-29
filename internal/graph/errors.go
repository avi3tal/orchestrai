package graph

import (
	"errors"
	"fmt"
)

var (
	// ErrAlreadyCompiled is returned when attempting to modify a compiled graph
	ErrAlreadyCompiled = errors.New("graph is already compiled and cannot be modified")

	// ErrInvalidNode is returned when a node fails validation
	ErrInvalidNode = errors.New("invalid node")

	// ErrDuplicateNode is returned when adding a node that already exists
	ErrDuplicateNode = errors.New("node with this ID already exists")

	// ErrNodeNotFound is returned when referencing a non-existent node
	ErrNodeNotFound = errors.New("node not found")

	// ErrCyclicDependency is returned when a cycle is detected in the graph
	ErrCyclicDependency = errors.New("cyclic dependency detected")

	// ErrNoEntryPoint is returned when validating a graph with no entry point
	ErrNoEntryPoint = errors.New("graph must have an entry point (edge from START)")

	// ErrNoEndPoint is returned when validating a graph with no end point
	ErrNoEndPoint = errors.New("graph must have at least one path to END")

	// ErrInvalidCondition is returned when an edge condition is invalid
	ErrInvalidCondition = errors.New("invalid edge condition")

	// ErrInvalidState is returned when the graph state is invalid
	ErrInvalidState = errors.New("invalid graph state")
)

// ValidationError represents an error that occurs during graph validation
type ValidationError struct {
	// Op is the operation that failed
	Op string
	// Node is the ID of the node involved (if any)
	Node string
	// Err is the underlying error
	Err error
}

func (e *ValidationError) Error() string {
	if e.Node != "" {
		return fmt.Sprintf("validation failed: %s: node '%s': %v", e.Op, e.Node, e.Err)
	}
	return fmt.Sprintf("validation failed: %s: %v", e.Op, e.Err)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// NewValidationError creates a new ValidationError
func NewValidationError(op string, node string, err error) error {
	return &ValidationError{
		Op:   op,
		Node: node,
		Err:  err,
	}
}

// RuntimeError represents an error that occurs during graph execution
type RuntimeError struct {
	// Op is the operation that failed
	Op string
	// Node is the ID of the node where the error occurred
	Node string
	// Err is the underlying error
	Err error
	// State is any relevant state information at the time of the error
	State string
}

func (e *RuntimeError) Error() string {
	if e.State != "" {
		return fmt.Sprintf("runtime error: %s: node '%s': %v (state: %s)",
			e.Op, e.Node, e.Err, e.State)
	}
	return fmt.Sprintf("runtime error: %s: node '%s': %v", e.Op, e.Node, e.Err)
}

func (e *RuntimeError) Unwrap() error {
	return e.Err
}

// NewRuntimeError creates a new RuntimeError
func NewRuntimeError(op string, node string, err error, state string) error {
	return &RuntimeError{
		Op:    op,
		Node:  node,
		Err:   err,
		State: state,
	}
}

// ExecutionError represents an error during the graph execution
type ExecutionError struct {
	// Phase is the execution phase where the error occurred
	Phase string
	// Node is the ID of the node being executed
	Node string
	// Err is the underlying error
	Err error
}

func (e *ExecutionError) Error() string {
	return fmt.Sprintf("execution error: %s: node '%s': %v", e.Phase, e.Node, e.Err)
}

func (e *ExecutionError) Unwrap() error {
	return e.Err
}

// NewExecutionError creates a new ExecutionError
func NewExecutionError(phase string, node string, err error) error {
	return &ExecutionError{
		Phase: phase,
		Node:  node,
		Err:   err,
	}
}

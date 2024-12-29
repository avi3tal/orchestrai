package graph

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avi3tal/orchestrai/internal/types"
	"github.com/google/uuid"
)

// Node represents a base node implementation
type Node struct {
	id          string
	name        string
	nodeType    string
	metadata    map[string]interface{}
	retryPolicy *types.RetryPolicy
	mu          sync.RWMutex
}

// NewNode creates a new base node
func NewNode(id, name, nodeType string) *Node {
	if id == "" {
		id = uuid.New().String()
	}

	return &Node{
		id:       id,
		name:     name,
		nodeType: nodeType,
		metadata: make(map[string]interface{}),
	}
}

// GetID returns the node's ID
func (n *Node) GetID() string {
	return n.id
}

// GetName returns the node's name
func (n *Node) GetName() string {
	return n.name
}

// GetType returns the node's type
func (n *Node) GetType() string {
	return n.nodeType
}

// SetMetadata sets a metadata value
func (n *Node) SetMetadata(key string, value interface{}) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.metadata[key] = value
}

// GetMetadata retrieves a metadata value
func (n *Node) GetMetadata(key string) (interface{}, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	v, ok := n.metadata[key]
	return v, ok
}

// SetRetryPolicy sets the retry policy for the node
func (n *Node) SetRetryPolicy(policy *types.RetryPolicy) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.retryPolicy = policy
}

// ExecutableNode represents a node that can process state
type ExecutableNode struct {
	*Node
	executor   types.StateTransformer
	validator  types.StateValidator
	middleware []types.NodeMiddleware
}

// NewExecutableNode creates a new executable node
func NewExecutableNode(id, name string, executor types.StateTransformer) *ExecutableNode {
	return &ExecutableNode{
		Node:     NewNode(id, name, "executable"),
		executor: executor,
	}
}

// Execute processes the state through the node
func (n *ExecutableNode) Execute(ctx context.Context, state types.State) (types.State, error) {
	if n.executor == nil {
		return nil, fmt.Errorf("no executor defined for node %s", n.id)
	}

	// Validate input state if validator is set
	if n.validator != nil {
		if err := n.validator(state); err != nil {
			return nil, fmt.Errorf("state validation failed: %w", err)
		}
	}

	// Apply middleware
	executor := n.executor
	for i := len(n.middleware) - 1; i >= 0; i-- {
		executor = n.middleware[i](executor)
	}

	// Execute with retry if policy exists
	var lastErr error
	attempts := 1
	if n.retryPolicy != nil {
		attempts = n.retryPolicy.MaxAttempts
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		newState, err := executor(ctx, state)
		if err == nil {
			return newState, nil
		}

		lastErr = err
		if attempt < attempts && n.retryPolicy != nil {
			delay := n.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				continue
			}
		}
	}

	return nil, fmt.Errorf("execution failed after %d attempts: %w", attempts, lastErr)
}

// AddMiddleware adds middleware to the node
func (n *ExecutableNode) AddMiddleware(middleware ...types.NodeMiddleware) {
	n.middleware = append(n.middleware, middleware...)
}

// SetValidator sets the state validator
func (n *ExecutableNode) SetValidator(validator types.StateValidator) {
	n.validator = validator
}

// calculateBackoff calculates the retry delay using exponential backoff
func (n *ExecutableNode) calculateBackoff(attempt int) time.Duration {
	if n.retryPolicy == nil {
		return 0
	}

	delay := float64(n.retryPolicy.InitialDelay)
	for i := 1; i < attempt; i++ {
		delay *= n.retryPolicy.BackoffMultiplier
		if delay > float64(n.retryPolicy.MaxDelay) {
			delay = float64(n.retryPolicy.MaxDelay)
			break
		}
	}

	return time.Duration(delay) * time.Millisecond
}

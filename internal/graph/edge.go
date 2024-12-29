package graph

import (
	"context"
	"fmt"
	"sync"

	"github.com/avi3tal/orchestrai/internal/types"
)

// Edge represents a connection between two nodes
type Edge struct {
	from      string
	to        string
	condition types.EdgeCondition
	metadata  map[string]interface{}
	mu        sync.RWMutex
}

// NewEdge creates a new edge
func NewEdge(from, to string) *Edge {
	return &Edge{
		from:     from,
		to:       to,
		metadata: make(map[string]interface{}),
	}
}

// From returns the source node ID
func (e *Edge) From() string {
	return e.from
}

// To returns the target node ID
func (e *Edge) To() string {
	return e.to
}

// SetCondition sets the edge condition
func (e *Edge) SetCondition(condition types.EdgeCondition) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.condition = condition
}

// GetCondition returns the edge condition if any
func (e *Edge) GetCondition() types.EdgeCondition {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.condition
}

// SetMetadata sets edge metadata
func (e *Edge) SetMetadata(key string, value interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.metadata[key] = value
}

// GetMetadata retrieves edge metadata
func (e *Edge) GetMetadata(key string) (interface{}, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	v, ok := e.metadata[key]
	return v, ok
}

// Evaluate evaluates the edge condition
func (e *Edge) Evaluate(ctx context.Context, state types.State) (bool, error) {
	if e.condition == nil {
		return true, nil
	}

	nextNode, err := e.condition(ctx, state)
	if err != nil {
		return false, fmt.Errorf("edge condition evaluation failed: %w", err)
	}

	return nextNode == e.to, nil
}

// Validate validates the edge configuration
func (e *Edge) Validate() error {
	if e.from == "" {
		return fmt.Errorf("edge must have a source node")
	}
	if e.to == "" {
		return fmt.Errorf("edge must have a target node")
	}
	if e.from == types.END {
		return fmt.Errorf("edge cannot start from END node")
	}
	if e.to == types.START {
		return fmt.Errorf("edge cannot point to START node")
	}
	return nil
}

package dag

import (
	"context"
	"fmt"
	"sync"
)

// MemoryCheckpointer is a simple in-memory implementation of Checkpointer
type MemoryCheckpointer[T State] struct {
	states map[string]T
	mu     sync.RWMutex
}

// NewMemoryCheckpointer creates a new memory checkpointer
func NewMemoryCheckpointer[T State]() *MemoryCheckpointer[T] {
	return &MemoryCheckpointer[T]{
		states: make(map[string]T),
	}
}

func (m *MemoryCheckpointer[T]) Save(ctx context.Context, config Config[T], state T) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.states[config.ThreadID] = state
	return nil
}

func (m *MemoryCheckpointer[T]) Load(ctx context.Context, config Config[T]) (T, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[config.ThreadID]
	if !exists {
		var zero T
		return zero, fmt.Errorf("no state found for thread %s", config.ThreadID)
	}
	return state, nil
}

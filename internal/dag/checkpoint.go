package dag

import (
	"context"
	"fmt"
	"sync"
)

// CheckpointData Extended checkpoint data
type CheckpointData[T GraphState[T]] struct {
	State       T
	Status      NodeExecutionStatus
	CurrentNode string
	Steps       int
}

// MemoryCheckpointer is a simple in-memory implementation of Checkpointer
type MemoryCheckpointer[T GraphState[T]] struct {
	checkpoints map[string]*CheckpointData[T]
	mu          sync.RWMutex
}

// NewMemoryCheckpointer creates a new memory checkpointer
func NewMemoryCheckpointer[T GraphState[T]]() *MemoryCheckpointer[T] {
	return &MemoryCheckpointer[T]{
		checkpoints: make(map[string]*CheckpointData[T]),
	}
}

func (m *MemoryCheckpointer[T]) Save(ctx context.Context, config Config[T], data *CheckpointData[T]) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkpoints[config.ThreadID] = data
	return nil
}

func (m *MemoryCheckpointer[T]) Load(ctx context.Context, config Config[T]) (*CheckpointData[T], error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cp, exists := m.checkpoints[config.ThreadID]
	if !exists {
		return nil, fmt.Errorf("no chckpoint found for thread %s", config.ThreadID)
	}
	return cp, nil
}

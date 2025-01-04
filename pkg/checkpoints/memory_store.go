package checkpoints

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avi3tal/orchestrai/pkg/state"
	"github.com/avi3tal/orchestrai/pkg/types"
)

type MemoryStore[T state.GraphState[T]] struct {
	checkpoints map[types.CheckpointKey]*types.Checkpoint[T]
	mu          sync.RWMutex
}

func NewMemoryStore[T state.GraphState[T]]() *MemoryStore[T] {
	return &MemoryStore[T]{
		checkpoints: make(map[types.CheckpointKey]*types.Checkpoint[T]),
	}
}

func (m *MemoryStore[T]) Save(_ context.Context, checkpoint types.Checkpoint[T]) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	checkpoint.Meta.UpdatedAt = time.Now()
	m.checkpoints[checkpoint.Key] = &checkpoint
	return nil
}

func (m *MemoryStore[T]) Load(_ context.Context, key types.CheckpointKey) (*types.Checkpoint[T], error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cp, exists := m.checkpoints[key]
	if !exists {
		return nil, fmt.Errorf("checkpoint not found: %v", key)
	}
	return cp, nil
}

func (m *MemoryStore[T]) Delete(_ context.Context, key types.CheckpointKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.checkpoints, key)
	return nil
}

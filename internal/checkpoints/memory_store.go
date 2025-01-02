package checkpoints

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avi3tal/orchestrai/internal/types"
)

type MemoryStore struct {
	checkpoints map[types.CheckpointKey]*types.Checkpoint
	mu          sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		checkpoints: make(map[types.CheckpointKey]*types.Checkpoint),
	}
}

func (m *MemoryStore) Save(_ context.Context, checkpoint types.Checkpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	checkpoint.Meta.UpdatedAt = time.Now()
	m.checkpoints[checkpoint.Key] = &checkpoint
	return nil
}

func (m *MemoryStore) Load(_ context.Context, key types.CheckpointKey) (*types.Checkpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cp, exists := m.checkpoints[key]
	if !exists {
		return nil, fmt.Errorf("checkpoint not found: %v", key)
	}
	return cp, nil
}

func (m *MemoryStore) Delete(_ context.Context, key types.CheckpointKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.checkpoints, key)
	return nil
}

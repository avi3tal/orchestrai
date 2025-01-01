package dag

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type CheckpointKey struct {
	GraphID  string
	ThreadID string
}

type CheckpointMeta struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	Steps     int
	Status    NodeExecutionStatus
}

type Checkpoint[T GraphState[T]] struct {
	Key    CheckpointKey
	Meta   CheckpointMeta
	State  T
	NodeID string
}

// CheckpointStore interface defines persistent storage operations
type CheckpointStore[T GraphState[T]] interface {
	Save(ctx context.Context, checkpoint Checkpoint[T]) error
	Load(ctx context.Context, key CheckpointKey) (*Checkpoint[T], error)
	List(ctx context.Context, graphID string) ([]CheckpointKey, error)
	Delete(ctx context.Context, key CheckpointKey) error
}

type MemoryStore[T GraphState[T]] struct {
	checkpoints map[CheckpointKey]*Checkpoint[T]
	mu          sync.RWMutex
}

func NewMemoryStore[T GraphState[T]]() *MemoryStore[T] {
	return &MemoryStore[T]{
		checkpoints: make(map[CheckpointKey]*Checkpoint[T]),
	}
}

func (m *MemoryStore[T]) Save(ctx context.Context, checkpoint Checkpoint[T]) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	checkpoint.Meta.UpdatedAt = time.Now()
	m.checkpoints[checkpoint.Key] = &checkpoint
	return nil
}

func (m *MemoryStore[T]) Load(ctx context.Context, key CheckpointKey) (*Checkpoint[T], error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cp, exists := m.checkpoints[key]
	if !exists {
		return nil, fmt.Errorf("checkpoint not found: %v", key)
	}
	return cp, nil
}

func (m *MemoryStore[T]) List(ctx context.Context, graphID string) ([]CheckpointKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []CheckpointKey
	for k := range m.checkpoints {
		if k.GraphID == graphID {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (m *MemoryStore[T]) Delete(ctx context.Context, key CheckpointKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.checkpoints, key)
	return nil
}

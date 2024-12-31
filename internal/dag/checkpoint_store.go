package dag

import (
	"context"
	"fmt"
	"sync"
)

type PendingExecutionError struct {
	NodeID string
}

func (e *PendingExecutionError) Error() string {
	return fmt.Sprintf("execution pending at node: %s", e.NodeID)
}

// CheckpointStore interface defines persistent storage operations
type CheckpointStore[T GraphState[T]] interface {
	Save(ctx context.Context, threadID string, data *CheckpointData[T]) error
	Load(ctx context.Context, threadID string) (*CheckpointData[T], error)
}

// StateCheckpointer manages execution state persistence
type StateCheckpointer[T GraphState[T]] struct {
	store CheckpointStore[T]
	mu    sync.RWMutex
}

func NewStateCheckpointer[T GraphState[T]](store CheckpointStore[T]) *StateCheckpointer[T] {
	return &StateCheckpointer[T]{
		store: store,
	}
}

func (sc *StateCheckpointer[T]) Save(ctx context.Context, config Config[T], data *CheckpointData[T]) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.store.Save(ctx, config.ThreadID, data)
}

func (sc *StateCheckpointer[T]) Load(ctx context.Context, config Config[T]) (*CheckpointData[T], error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.store.Load(ctx, config.ThreadID)
}

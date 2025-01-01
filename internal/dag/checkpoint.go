package dag

import (
	"context"
	"time"
)

// CheckpointData Extended checkpoint data
type CheckpointData[T GraphState[T]] struct {
	State       T
	Status      NodeExecutionStatus
	CurrentNode string
	Steps       int
	NodeQueue   []string
}

// StateCheckpointer manages execution state persistence
type StateCheckpointer[T GraphState[T]] struct {
	store CheckpointStore[T]
}

func NewStateCheckpointer[T GraphState[T]](store CheckpointStore[T]) *StateCheckpointer[T] {
	return &StateCheckpointer[T]{
		store: store,
	}
}

func (sc *StateCheckpointer[T]) Save(ctx context.Context, config Config[T], data *CheckpointData[T]) error {
	key := CheckpointKey{
		GraphID:  config.GraphID,
		ThreadID: config.ThreadID,
	}

	cp := Checkpoint[T]{
		Key: key,
		Meta: CheckpointMeta{
			CreatedAt: time.Now(),
			Steps:     data.Steps,
			Status:    data.Status,
			NodeQueue: data.NodeQueue,
		},
		State:  data.State,
		NodeID: data.CurrentNode,
	}

	return sc.store.Save(ctx, cp)
}

func (sc *StateCheckpointer[T]) Load(ctx context.Context, config Config[T]) (*CheckpointData[T], error) {
	key := CheckpointKey{
		GraphID:  config.GraphID,
		ThreadID: config.ThreadID,
	}

	cp, err := sc.store.Load(ctx, key)
	if err != nil {
		return nil, err
	}

	data := &CheckpointData[T]{
		State:       cp.State,
		CurrentNode: cp.NodeID,
		Status:      cp.Meta.Status,
		Steps:       cp.Meta.Steps,
		NodeQueue:   cp.Meta.NodeQueue,
	}

	return data, nil
}

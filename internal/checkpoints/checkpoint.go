package checkpoints

import (
	"context"
	"time"

	"github.com/avi3tal/orchestrai/internal/state"
	"github.com/avi3tal/orchestrai/internal/types"
)

// StateCheckpointer manages execution state persistence
type StateCheckpointer[T state.GraphState[T]] struct {
	store types.CheckpointStore[T]
}

func NewStateCheckpointer[T state.GraphState[T]](store types.CheckpointStore[T]) *StateCheckpointer[T] {
	return &StateCheckpointer[T]{
		store: store,
	}
}

func (sc *StateCheckpointer[T]) Save(ctx context.Context, config types.Config[T], data *types.DataPoint[T]) error {
	key := types.CheckpointKey{
		GraphID:  config.GraphID,
		ThreadID: config.ThreadID,
	}

	cp := types.Checkpoint[T]{
		Key: key,
		Meta: types.CheckpointMeta{
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

func (sc *StateCheckpointer[T]) Load(ctx context.Context, config types.Config[T]) (*types.DataPoint[T], error) {
	key := types.CheckpointKey{
		GraphID:  config.GraphID,
		ThreadID: config.ThreadID,
	}

	cp, err := sc.store.Load(ctx, key)
	if err != nil {
		return nil, err
	}

	data := &types.DataPoint[T]{
		State:       cp.State,
		CurrentNode: cp.NodeID,
		Status:      cp.Meta.Status,
		Steps:       cp.Meta.Steps,
		NodeQueue:   cp.Meta.NodeQueue,
	}

	return data, nil
}

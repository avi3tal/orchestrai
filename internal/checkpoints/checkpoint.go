package checkpoints

import (
	"context"
	"fmt"
	"time"

	"github.com/avi3tal/orchestrai/internal/types"
)

// StateCheckpointer manages execution state persistence
type StateCheckpointer struct {
	store types.CheckpointStore
}

func NewStateCheckpointer(store types.CheckpointStore) *StateCheckpointer {
	return &StateCheckpointer{
		store: store,
	}
}

func (sc *StateCheckpointer) Save(ctx context.Context, config types.Config, data *types.DataPoint) error {
	key := types.CheckpointKey{
		GraphID:  config.GraphID,
		ThreadID: config.ThreadID,
	}

	cp := types.Checkpoint{
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

	if err := sc.store.Save(ctx, cp); err != nil {
		return fmt.Errorf("failed to save checkpoint for GraphID %s and ThreadID %s: %w", key.GraphID, key.ThreadID, err)
	}
	return nil
}

func (sc *StateCheckpointer) Load(ctx context.Context, config types.Config) (*types.DataPoint, error) {
	key := types.CheckpointKey{
		GraphID:  config.GraphID,
		ThreadID: config.ThreadID,
	}

	cp, err := sc.store.Load(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to load checkpoint for GraphID %s and ThreadID %s: %w", key.GraphID, key.ThreadID, err)
	}

	data := &types.DataPoint{
		State:       cp.State,
		CurrentNode: cp.NodeID,
		Status:      cp.Meta.Status,
		Steps:       cp.Meta.Steps,
		NodeQueue:   cp.Meta.NodeQueue,
	}

	return data, nil
}

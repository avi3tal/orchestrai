package dag

import (
	"context"
	"time"
)

func executeNode[T GraphState[T]](
	ctx context.Context,
	node NodeSpec[T],
	state T,
	config Config[T],
) (NodeResponse[T], error) {
	maxAttempts := DefaultMaxRetries
	if node.RetryPolicy != nil {
		maxAttempts = node.RetryPolicy.MaxAttempts
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 && node.RetryPolicy != nil {
			time.Sleep(time.Duration(node.RetryPolicy.Delay) * time.Second)
		}

		resp, err := node.Function(ctx, state, config)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return NodeResponse[T]{}, lastErr
}

func saveCheckpoint[T GraphState[T]](
	ctx context.Context,
	state T,
	node string,
	status NodeExecutionStatus,
	steps int,
	config Config[T],
) error {
	if config.Checkpointer == nil {
		return nil
	}

	data := &CheckpointData[T]{
		State:       state,
		CurrentNode: node,
		Status:      status,
		Steps:       steps,
	}
	return config.Checkpointer.Save(ctx, config, data)
}

func loadOrInitCheckpoint[T GraphState[T]](
	ctx context.Context,
	entryPoint string,
	initialState T,
	config Config[T],
) CheckpointData[T] {
	data := CheckpointData[T]{
		State:       initialState,
		CurrentNode: entryPoint,
		Status:      StatusReady,
		Steps:       0,
	}

	if config.Checkpointer == nil {
		return data
	}

	// Load the last checkpoint if available
	if checkpoint, err := config.Checkpointer.Load(ctx, config); err == nil {
		data.CurrentNode = checkpoint.CurrentNode
		data.State = checkpoint.State.Merge(initialState)
		data.Steps = checkpoint.Steps
	}

	return data
}

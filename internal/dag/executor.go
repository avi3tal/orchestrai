package dag

import (
	"context"
	"fmt"
	"time"
)

const DefaultMaxRetries = 1

type NextNode struct {
	Target string // Next node to execute
	Then   string // Optional post-processing node
}

type NextNodeInfo[T GraphState[T]] struct {
	Target    NextNode
	PrevState T
}

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
	nodeQueue ...string,
) error {
	if config.Checkpointer == nil {
		return nil
	}

	data := &CheckpointData[T]{
		State:       state,
		CurrentNode: node,
		Status:      status,
		Steps:       steps,
		NodeQueue:   nodeQueue,
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
		NodeQueue:   []string{entryPoint},
	}

	if config.Checkpointer == nil {
		return data
	}

	// Load the last checkpoint if available
	if checkpoint, err := config.Checkpointer.Load(ctx, config); err == nil {
		data.State = checkpoint.State.Merge(initialState)
		data.Steps = checkpoint.Steps
		// Restore the CurrentNode and node queue if the checkpoint is pending
		// This will resume the execution from the last pending node
		// and skip the nodes that have already
		if checkpoint.Status == StatusPending {
			data.CurrentNode = checkpoint.CurrentNode
			data.NodeQueue = checkpoint.NodeQueue
		}
	}

	return data
}

func checkExecutionLimits[T GraphState[T]](ctx context.Context, steps int, config Config[T]) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("execution cancelled: %w", ctx.Err())
	default:
	}

	if config.MaxSteps > 0 && steps >= config.MaxSteps {
		return fmt.Errorf("max steps reached (%d)", config.MaxSteps)
	}

	return nil
}

func execute[T GraphState[T]](
	ctx context.Context,
	graph *Graph[T],
	initialState T,
	config Config[T],
) (T, error) {
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(config.Timeout)*time.Second)
		defer cancel()
	}

	// Load or initialize the state and checkpoint
	checkpoint := loadOrInitCheckpoint(ctx, graph.entryPoint, initialState, config)
	state := checkpoint.State
	steps := checkpoint.Steps
	nodeQueue := checkpoint.NodeQueue

	for len(nodeQueue) > 0 {
		if err := checkExecutionLimits(ctx, steps, config); err != nil {
			return state, err
		}

		// Pop next node
		current := nodeQueue[0]
		nodeQueue = nodeQueue[1:]

		if current == END {
			continue
		}

		// Execute current node
		node, exists := graph.nodes[current]
		if !exists {
			return state, fmt.Errorf("node %s not found", current)
		}

		resp, err := executeNode(ctx, node, state, config)
		if err != nil {
			return state, err
		}
		state = state.Merge(resp.State)

		// Save the checkpoint after executing the node
		if err = saveCheckpoint(
			ctx, state, current, resp.Status, steps, config, nodeQueue...,
		); err != nil {
			return state, err
		}

		// If the node is pending, return the current state and the pending error
		if resp.Status == StatusPending {
			return state, nil
		}

		// Queue next nodes
		next, err := getNextNode(ctx, graph, current, state, config)
		if err != nil {
			return state, err
		}

		// Queue Then node if exists
		if next.Target != END {
			nodeQueue = append(nodeQueue, next.Target)
		}
		if next.Then != "" && next.Then != END {
			nodeQueue = append(nodeQueue, next.Then)
		}

		steps++
	}

	return state, nil
}

func getNextNode[T GraphState[T]](
	ctx context.Context,
	graph *Graph[T],
	currentNode string,
	state T,
	config Config[T],
) (NextNode, error) {
	// Check branches first
	for _, branch := range graph.branches[currentNode] {
		if target := branch.Path(ctx, state, config); target != "" {
			return NextNode{
				Target: target,
				Then:   branch.Then,
			}, nil
		}
	}

	// Fall back to direct edge
	for _, edge := range graph.edges {
		if edge.From == currentNode {
			return NextNode{Target: edge.To}, nil
		}
	}

	return NextNode{}, fmt.Errorf("no transition from node: %s", currentNode)
}

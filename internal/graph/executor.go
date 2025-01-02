package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/avi3tal/orchestrai/internal/state"
	"github.com/avi3tal/orchestrai/internal/types"
)

const DefaultMaxRetries = 1

type NextNode struct {
	Target string // Next node to execute
	Then   string // Optional post-processing node
}

type NextNodeInfo struct {
	Target    NextNode
	PrevState state.GraphState
}

func executeNode(
	ctx context.Context,
	node NodeSpec,
	state state.GraphState,
	config types.Config,
) (NodeResponse, error) {
	maxAttempts := DefaultMaxRetries
	if node.RetryPolicy != nil {
		maxAttempts = node.RetryPolicy.MaxAttempts
	}

	var lastErr error
	for attempt := range maxAttempts {
		if attempt > 0 && node.RetryPolicy != nil {
			time.Sleep(time.Duration(node.RetryPolicy.Delay) * time.Second)
		}

		resp, err := node.Function(ctx, state, config)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return NodeResponse{}, fmt.Errorf("failed to execute node %s: %w", node.Name, lastErr)
}

func saveCheckpoint(
	ctx context.Context,
	state state.GraphState,
	node string,
	status types.NodeExecutionStatus,
	steps int,
	config types.Config,
	nodeQueue ...string,
) error {
	if config.Checkpointer == nil {
		return nil
	}

	data := &types.DataPoint{
		State:       state,
		CurrentNode: node,
		Status:      status,
		Steps:       steps,
		NodeQueue:   nodeQueue,
	}
	if err := config.Checkpointer.Save(ctx, config, data); err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}
	return nil
}

func loadOrInitCheckpoint(
	ctx context.Context,
	entryPoint string,
	initialState state.GraphState,
	config types.Config,
) types.DataPoint {
	data := types.DataPoint{
		State:       initialState,
		CurrentNode: entryPoint,
		Status:      types.StatusReady,
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
		if checkpoint.Status == types.StatusPending {
			data.CurrentNode = checkpoint.CurrentNode
			data.NodeQueue = checkpoint.NodeQueue
		}
	}

	return data
}

func checkExecutionLimits(ctx context.Context, steps int, config types.Config) error {
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

func execute(
	ctx context.Context,
	graph *Graph,
	initialState state.GraphState,
	config types.Config,
) (state.GraphState, error) {
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(config.Timeout)*time.Second)
		defer cancel()
	}

	// Load or initialize the state and checkpoint
	checkpoint := loadOrInitCheckpoint(ctx, graph.entryPoint, initialState, config)
	st := checkpoint.State
	steps := checkpoint.Steps
	nodeQueue := checkpoint.NodeQueue

	for len(nodeQueue) > 0 {
		if err := checkExecutionLimits(ctx, steps, config); err != nil {
			return st, err
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
			return st, fmt.Errorf("node %s not found", current)
		}

		resp, err := executeNode(ctx, node, st, config)
		if err != nil {
			return st, err
		}
		st = st.Merge(resp.State)

		// Save the checkpoint after executing the node
		if err = saveCheckpoint(
			ctx, st, current, resp.Status, steps, config, nodeQueue...,
		); err != nil {
			return st, err
		}

		// If the node is pending, return the current state and the pending error
		if resp.Status == types.StatusPending {
			return st, nil
		}

		// Queue next nodes
		next, err := getNextNode(ctx, graph, current, st, config)
		if err != nil {
			return st, err
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

	return st, nil
}

func getNextNode(
	ctx context.Context,
	graph *Graph,
	currentNode string,
	state state.GraphState,
	config types.Config,
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

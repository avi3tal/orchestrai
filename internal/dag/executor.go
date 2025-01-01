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
	currentNode := checkpoint.CurrentNode
	state := checkpoint.State
	steps := checkpoint.Steps
	var nextInfo *NextNodeInfo[T]

	for currentNode != END {
		if config.Debug {
			// TODO - add logging
			fmt.Printf("Executing node: %s (Step: %d)\n", currentNode, steps)
		}

		if config.MaxSteps > 0 && steps >= config.MaxSteps {
			return state, fmt.Errorf("max steps reached")
		}

		// Execute Then node if pending from previous iteration
		if nextInfo != nil && nextInfo.Target.Then != "" {
			thenNode, exists := graph.nodes[nextInfo.Target.Then]
			if !exists {
				return state, fmt.Errorf("then node %s not found", nextInfo.Target.Then)
			}

			resp, err := executeNode(ctx, thenNode, state, config)
			if err != nil {
				return state, err
			}
			state = state.Merge(resp.State)
			nextInfo = nil
		}

		node, exists := graph.nodes[currentNode]
		if !exists {
			return state, fmt.Errorf("node %s not found", currentNode)
		}

		// Execute the current node
		resp, err := executeNode(ctx, node, state, config)
		if err != nil {
			return state, err
		}

		// Merge the response state with the current state
		state = state.Merge(resp.State)

		// Save the checkpoint after executing the node
		if err = saveCheckpoint(ctx, state, currentNode, resp.Status, steps, config); err != nil {
			return state, err
		}

		// If the node is pending, return the current state and the pending error
		if resp.Status == StatusPending {
			return state, nil
		}

		// Determine the next node
		nextNode, err := getNextNode(ctx, graph, currentNode, state, config)
		if err != nil {
			return state, err
		}

		// Store next node info if it has Then
		if nextNode.Then != "" && nextNode.Then != END {
			nextInfo = &NextNodeInfo[T]{
				Target:    nextNode,
				PrevState: state,
			}
		}

		if config.Debug {
			fmt.Printf("Transition: %s -> %s\n", currentNode, nextNode)
		}

		currentNode = nextNode.Target
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

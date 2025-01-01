package dag

import (
	"context"
	"fmt"
	"time"
)

const DefaultMaxRetries = 1

// NodeExecutionStatus represents the current state of node execution
type NodeExecutionStatus string

const (
	StatusCompleted NodeExecutionStatus = "completed"
	StatusPending   NodeExecutionStatus = "pending" // Waiting for user input
	StatusFailed    NodeExecutionStatus = "failed"
	StatusReady     NodeExecutionStatus = "ready" // Ready to execute
)

// NodeResponse encapsulates the execution result
type NodeResponse[T GraphState[T]] struct {
	State  T
	Status NodeExecutionStatus
}

type NextNode struct {
	Target string // Next node to execute
	Then   string // Optional post-processing node
}

type NextNodeInfo[T GraphState[T]] struct {
	Target    NextNode
	PrevState T
}

// CompiledGraph represents a validated and executable graph
type CompiledGraph[T GraphState[T]] struct {
	graph  *Graph[T]
	config Config[T]
}

// Compile validates and compiles the graph for execution
func (g *Graph[T]) Compile(opt ...CompilationOption[T]) (*CompiledGraph[T], error) {
	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("graph validation failed: %w", err)
	}

	g.compiled = true

	config := NewConfig[T](g.graphID, opt...)

	return &CompiledGraph[T]{
		graph:  g,
		config: config,
	}, nil
}

func (cg *CompiledGraph[T]) saveCheckpoint(ctx context.Context, state T, node string, status NodeExecutionStatus, steps int) error {
	if cg.config.Checkpointer == nil {
		return nil
	}

	data := &CheckpointData[T]{
		State:       state,
		CurrentNode: node,
		Status:      status,
		Steps:       steps,
	}
	return cg.config.Checkpointer.Save(ctx, cg.config, data)
}

func (cg *CompiledGraph[T]) loadOrInitCheckpoint(ctx context.Context, initialState T) CheckpointData[T] {
	data := CheckpointData[T]{
		State:       initialState,
		CurrentNode: cg.graph.entryPoint,
		Status:      StatusReady,
		Steps:       0,
	}

	if cg.config.Checkpointer == nil {
		return data
	}

	// Load the last checkpoint if available
	if checkpoint, err := cg.config.Checkpointer.Load(ctx, cg.config); err == nil {
		data.CurrentNode = checkpoint.CurrentNode
		data.State = checkpoint.State.Merge(initialState)
		data.Steps = checkpoint.Steps
	}

	return data
}

func (cg *CompiledGraph[T]) Run(ctx context.Context, initialState T, opt ...ExecutionOption[T]) (T, error) {
	// Apply execution options
	for _, o := range opt {
		o(&cg.config)
	}

	if cg.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(cg.config.Timeout)*time.Second)
		defer cancel()
	}

	// Load or initialize the state and checkpoint
	checkpoint := cg.loadOrInitCheckpoint(ctx, initialState)
	currentNode := checkpoint.CurrentNode
	state := checkpoint.State
	steps := checkpoint.Steps
	var nextInfo *NextNodeInfo[T]

	for currentNode != END {
		if cg.config.Debug {
			// TODO - add logging
			fmt.Printf("Executing node: %s (Step: %d)\n", currentNode, steps)
		}

		if cg.config.MaxSteps > 0 && steps >= cg.config.MaxSteps {
			return state, fmt.Errorf("max steps reached")
		}

		// Execute Then node if pending from previous iteration
		if nextInfo != nil && nextInfo.Target.Then != "" {
			thenNode, exists := cg.graph.nodes[nextInfo.Target.Then]
			if !exists {
				return state, fmt.Errorf("then node %s not found", nextInfo.Target.Then)
			}

			resp, err := cg.executeNode(ctx, thenNode, state)
			if err != nil {
				return state, err
			}
			state = state.Merge(resp.State)
			nextInfo = nil
		}

		node, exists := cg.graph.nodes[currentNode]
		if !exists {
			return state, fmt.Errorf("node %s not found", currentNode)
		}

		// Execute the current node
		resp, err := cg.executeNode(ctx, node, state)
		if err != nil {
			return state, err
		}

		// Merge the response state with the current state
		state = state.Merge(resp.State)

		// Save the checkpoint after executing the node
		if err = cg.saveCheckpoint(ctx, state, currentNode, resp.Status, steps); err != nil {
			return state, err
		}

		// If the node is pending, return the current state and the pending error
		if resp.Status == StatusPending {
			return state, nil
		}

		// Determine the next node
		nextNode, err := cg.getNextNode(ctx, currentNode, state)
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

		if cg.config.Debug {
			fmt.Printf("Transition: %s -> %s\n", currentNode, nextNode)
		}

		currentNode = nextNode.Target
		steps++
	}

	return state, nil
}

func (cg *CompiledGraph[T]) getNextNode(ctx context.Context, currentNode string, state T) (NextNode, error) {
	// Check branches first
	for _, branch := range cg.graph.branches[currentNode] {
		if target := branch.Path(ctx, state, cg.config); target != "" {
			return NextNode{
				Target: target,
				Then:   branch.Then,
			}, nil
		}
	}

	// Fall back to direct edge
	for _, edge := range cg.graph.edges {
		if edge.From == currentNode {
			return NextNode{Target: edge.To}, nil
		}
	}

	return NextNode{}, fmt.Errorf("no transition from node: %s", currentNode)
}

func (cg *CompiledGraph[T]) executeNode(ctx context.Context, node NodeSpec[T], state T) (NodeResponse[T], error) {
	maxAttempts := DefaultMaxRetries
	if node.RetryPolicy != nil {
		maxAttempts = node.RetryPolicy.MaxAttempts
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 && node.RetryPolicy != nil {
			time.Sleep(time.Duration(node.RetryPolicy.Delay) * time.Second)
		}

		resp, err := node.Function(ctx, state, cg.config)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return NodeResponse[T]{}, lastErr
}

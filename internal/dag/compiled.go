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

func (cg *CompiledGraph[T]) Run(ctx context.Context, initialState T, opt ...ExecutionOption[T]) (T, error) {
	// Apply execution options
	config := cg.config.Clone()
	for _, o := range opt {
		o(&config)
	}

	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(config.Timeout)*time.Second)
		defer cancel()
	}

	// Load or initialize the state and checkpoint
	checkpoint := loadOrInitCheckpoint(ctx, cg.graph.entryPoint, initialState, config)
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
			thenNode, exists := cg.graph.nodes[nextInfo.Target.Then]
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

		node, exists := cg.graph.nodes[currentNode]
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

package dag

import (
	"context"
	"fmt"
	"time"
)

// CompiledGraph represents a validated and executable graph
type CompiledGraph[T State] struct {
	graph  *Graph[T]
	config Config[T]
}

// Compile validates and compiles the graph for execution
func (g *Graph[T]) Compile(config Config[T]) (*CompiledGraph[T], error) {
	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("graph validation failed: %w", err)
	}

	g.compiled = true

	return &CompiledGraph[T]{
		graph:  g,
		config: config,
	}, nil
}

// Run executes the compiled graph from the given initial state
func (cg *CompiledGraph[T]) Run(ctx context.Context, initialState T) (T, error) {
	var state T

	// Setup context with timeout if specified
	if cg.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(cg.config.Timeout)*time.Second)
		defer cancel()
	}

	// Initialize state
	state = initialState

	// Save initial state to checkpointer if available
	if cg.config.Checkpointer != nil {
		if err := cg.config.Checkpointer.Save(ctx, cg.config, state); err != nil {
			return state, fmt.Errorf("failed to save initial state: %w", err)
		}
	}

	currentNode := cg.graph.entryPoint
	steps := 0

	for currentNode != END {
		if cg.config.Debug {
			fmt.Printf("Executing node: %s (Step: %d)\n", currentNode, steps)
		}

		// Check step limit
		if cg.config.MaxSteps > 0 && steps >= cg.config.MaxSteps {
			return state, fmt.Errorf("max steps reached")
		}

		// Execute node
		node, exists := cg.graph.nodes[currentNode]
		if !exists {
			return state, fmt.Errorf("node %s not found", currentNode)
		}

		// Execute with retry policy if specified
		var err error
		if node.RetryPolicy != nil {
			state, err = cg.executeWithRetry(ctx, node, state)
		} else {
			state, err = node.Function(ctx, state, cg.config)
		}

		if err != nil {
			return state, fmt.Errorf("error executing node %s: %w", currentNode, err)
		}

		// Save state if checkpointer is available
		if cg.config.Checkpointer != nil {
			if err := cg.config.Checkpointer.Save(ctx, cg.config, state); err != nil {
				return state, fmt.Errorf("failed to save state: %w", err)
			}
		}

		// Determine next node - branches have precedence over edges
		nextNode := ""

		// First check branches
		if branches, hasBranches := cg.graph.branches[currentNode]; hasBranches {
			for _, branch := range branches {
				branchTarget := branch.Path(ctx, state, cg.config)
				if branchTarget != "" {
					nextNode = branchTarget

					// Execute 'then' node if specified
					if branch.Then != "" && branch.Then != END {
						thenNode, exists := cg.graph.nodes[branch.Then]
						if !exists {
							return state, fmt.Errorf("then node %s not found", branch.Then)
						}

						thenState, err := thenNode.Function(ctx, state, cg.config)
						if err != nil {
							return state, fmt.Errorf("error executing then node %s: %w", branch.Then, err)
						}
						state = thenState

						// Save state after 'then' node if checkpointer is available
						if cg.config.Checkpointer != nil {
							if err := cg.config.Checkpointer.Save(ctx, cg.config, state); err != nil {
								return state, fmt.Errorf("failed to save state after then node: %w", err)
							}
						}
					}
					break
				}
			}
		}

		// If no branch was taken, check edges
		if nextNode == "" {
			for _, edge := range cg.graph.edges {
				if edge.From == currentNode {
					nextNode = edge.To
					break
				}
			}
		}

		// Validate we have a next node
		if nextNode == "" {
			return state, fmt.Errorf("no valid transition found for node %s", currentNode)
		}

		if cg.config.Debug {
			fmt.Printf("Transition: %s -> %s\n", currentNode, nextNode)
		}

		currentNode = nextNode
		steps++
	}

	return state, nil
}

func (cg *CompiledGraph[T]) executeWithRetry(ctx context.Context, node NodeSpec[T], state T) (T, error) {
	var lastErr error

	for attempt := 0; attempt < node.RetryPolicy.MaxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(node.RetryPolicy.Delay) * time.Second)
		}

		newState, err := node.Function(ctx, state, cg.config)
		if err == nil {
			return newState, nil
		}

		lastErr = err
	}

	return state, fmt.Errorf("max retry attempts reached: %w", lastErr)
}

package graph

import (
	"context"
	"fmt"

	"github.com/avi3tal/orchestrai/internal/state"
	"github.com/avi3tal/orchestrai/internal/types"
)

// CompiledGraph represents a validated and executable graph
type CompiledGraph[T state.GraphState[T]] struct {
	graph  *Graph[T]
	config types.Config[T]
}

// Compile validates and compiles the graph for execution
func (g *Graph[T]) Compile(opt ...CompilationOption[T]) (*CompiledGraph[T], error) {
	// Validate the graph before compiling
	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("graph validation failed: %w", err)
	}
	// Mark the graph as compiled
	g.compiled = true

	// Create a new configuration for the compiled graph
	config := NewConfig[T](g.graphID, opt...)

	return &CompiledGraph[T]{
		graph:  g,
		config: config,
	}, nil
}

// Run executes the compiled graph with the provided initial state
func (cg *CompiledGraph[T]) Run(ctx context.Context, initialState T, opt ...ExecutionOption[T]) (T, error) {
	// Apply execution options
	config := cg.config.Clone()
	for _, o := range opt {
		o(&config)
	}
	// Execute the compiled graph
	return execute(ctx, cg.graph, initialState, config)
}

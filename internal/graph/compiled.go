package graph

import (
	"context"
	"fmt"

	"github.com/avi3tal/orchestrai/internal/state"
	"github.com/avi3tal/orchestrai/internal/types"
)

// CompiledGraph represents a validated and executable graph
type CompiledGraph struct {
	graph  *Graph
	config types.Config
}

// Compile validates and compiles the graph for execution
func (g *Graph) Compile(opt ...CompilationOption) (*CompiledGraph, error) {
	// Validate the graph before compiling
	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("graph validation failed: %w", err)
	}
	// Mark the graph as compiled
	g.compiled = true

	// Create a new configuration for the compiled graph
	config := NewConfig(g.graphID, opt...)

	return &CompiledGraph{
		graph:  g,
		config: config,
	}, nil
}

// Run executes the compiled graph with the provided initial state
func (cg *CompiledGraph) Run(ctx context.Context, initialState state.GraphState, opt ...ExecutionOption) (state.GraphState, error) {
	// Apply execution options
	config := cg.config.Clone()
	for _, o := range opt {
		o(&config)
	}
	// Execute the compiled graph
	return execute(ctx, cg.graph, initialState, config)
}

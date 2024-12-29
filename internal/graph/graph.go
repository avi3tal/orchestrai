package graph

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/avi3tal/orchestrai/internal/types"
	"github.com/google/uuid"
)

// Graph represents a directed acyclic graph workflow
type Graph struct {
	// Unique identifier for this graph instance
	id string

	// Version of the graph
	version string

	// Graph components
	nodes    map[string]types.Node
	edges    map[string][]types.Edge
	branches map[string][]types.Edge

	// Entry and exit points
	entryPoints map[string]bool
	exitPoints  map[string]bool

	// Runtime components
	checkpointer types.Checkpointer
	store        types.Store

	// Configuration
	maxConcurrency int
	debug          bool
	errorHandler   func(error) error

	// Metadata
	metadata map[string]interface{}

	// Build state
	compiled atomic.Bool
	mu       sync.RWMutex
}

// NewGraph creates a new empty graph
func NewGraph(opts ...Option) *Graph {
	g := &Graph{
		id:             uuid.New().String(),
		version:        "1.0.0",
		nodes:          make(map[string]types.Node),
		edges:          make(map[string][]types.Edge),
		branches:       make(map[string][]types.Edge),
		entryPoints:    make(map[string]bool),
		exitPoints:     make(map[string]bool),
		metadata:       make(map[string]interface{}),
		maxConcurrency: 10, // default value
	}

	// Apply options
	for _, opt := range opts {
		opt(g)
	}

	return g
}

// AddNode adds a new node to the graph
func (g *Graph) AddNode(node types.Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.compiled.Load() {
		return ErrAlreadyCompiled
	}

	if node == nil {
		return NewValidationError("AddNode", "", ErrInvalidNode)
	}

	id := node.GetID()
	if id == "" {
		return NewValidationError("AddNode", "", fmt.Errorf("%w: empty node ID", ErrInvalidNode))
	}

	if id == types.START || id == types.END {
		return NewValidationError("AddNode", id, fmt.Errorf("%w: reserved node ID", ErrInvalidNode))
	}

	if _, exists := g.nodes[id]; exists {
		return NewValidationError("AddNode", id, ErrDuplicateNode)
	}

	g.nodes[id] = node
	return nil
}

// AddEdge creates a direct edge between two nodes
func (g *Graph) AddEdge(from, to string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.compiled.Load() {
		return ErrAlreadyCompiled
	}

	if from != types.START && !g.nodeExists(from) {
		return NewValidationError("AddEdge", from, ErrNodeNotFound)
	}

	if to != types.END && !g.nodeExists(to) {
		return NewValidationError("AddEdge", to, ErrNodeNotFound)
	}

	edge := types.Edge{
		From: from,
		To:   to,
	}

	g.edges[from] = append(g.edges[from], edge)

	// Update entry/exit points
	if from == types.START {
		g.entryPoints[to] = true
	}
	if to == types.END {
		g.exitPoints[from] = true
	}

	return nil
}

// AddConditionalEdge creates an edge with a condition
func (g *Graph) AddConditionalEdge(from string, condition types.EdgeCondition, to string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.compiled.Load() {
		return ErrAlreadyCompiled
	}

	if condition == nil {
		return NewValidationError("AddConditionalEdge", from, ErrInvalidCondition)
	}

	if from != types.START && !g.nodeExists(from) {
		return NewValidationError("AddConditionalEdge", from, ErrNodeNotFound)
	}

	if to != types.END && !g.nodeExists(to) {
		return NewValidationError("AddConditionalEdge", to, ErrNodeNotFound)
	}

	edge := types.Edge{
		From:      from,
		To:        to,
		Condition: condition,
	}

	g.branches[from] = append(g.branches[from], edge)

	// Update entry/exit points
	if from == types.START {
		g.entryPoints[to] = true
	}
	if to == types.END {
		g.exitPoints[from] = true
	}

	return nil
}

// Validate ensures the graph is properly constructed
func (g *Graph) Validate() error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Validate entry points
	if len(g.entryPoints) == 0 {
		return NewValidationError("Validate", "", ErrNoEntryPoint)
	}

	// Validate exit points
	if len(g.exitPoints) == 0 {
		return NewValidationError("Validate", "", ErrNoEndPoint)
	}

	// Validate cycles (use a snapshot for cycle detection if necessary)
	if err := g.detectCycles(); err != nil {
		return NewValidationError("Validate", "", err)
	}

	// Validate individual nodes
	for id, node := range g.nodes {
		if validator, ok := node.(types.Validator); ok {
			if err := validator.Validate(); err != nil {
				return NewValidationError("Validate", id, err)
			}
		}
	}

	return nil
}

// Compile creates an immutable compiled version of the graph
func (g *Graph) Compile(opts ...types.CompileOption) (types.CompiledGraph, error) {
	// Validate graph
	if err := g.Validate(); err != nil {
		return nil, err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.compiled.Load() {
		return nil, ErrAlreadyCompiled
	}

	// Apply compile options
	options := &types.CompileOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Create compiled graph
	compiled := &CompiledGraph{
		id:              g.id,
		version:         g.version,
		nodes:           g.nodes,
		edges:           g.edges,
		branches:        g.branches,
		checkpointer:    g.checkpointer,
		store:           g.store,
		interruptBefore: options.InterruptBefore,
		interruptAfter:  options.InterruptAfter,
		debug:           options.Debug,
	}

	g.compiled.Store(true)
	return compiled, nil
}

// nodeExists checks if a node exists in the graph
func (g *Graph) nodeExists(id string) bool {
	_, exists := g.nodes[id]
	return exists
}

// detectCycles checks for cycles in the graph
func (g *Graph) detectCycles() error {
	visited := make(map[string]bool)
	stack := make(map[string]bool)

	var dfs func(node string) error
	dfs = func(node string) error {
		visited[node] = true
		stack[node] = true

		// Check direct edges
		for _, edge := range g.edges[node] {
			if !visited[edge.To] {
				if err := dfs(edge.To); err != nil {
					return err
				}
			} else if stack[edge.To] {
				return ErrCyclicDependency
			}
		}

		// Check conditional edges
		for _, edge := range g.branches[node] {
			if !visited[edge.To] {
				if err := dfs(edge.To); err != nil {
					return err
				}
			} else if stack[edge.To] {
				return ErrCyclicDependency
			}
		}

		stack[node] = false
		return nil
	}

	// Start DFS from entry points
	for node := range g.entryPoints {
		if !visited[node] {
			if err := dfs(node); err != nil {
				return err
			}
		}
	}

	return nil
}

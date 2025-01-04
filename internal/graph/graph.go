package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/avi3tal/orchestrai/pkg/state"
	"github.com/avi3tal/orchestrai/pkg/types"
)

// Constants for special nodes
const (
	START            = "START"
	END              = "END"
	defaultGraphName = "graph"
)

// NodeSpec represents a node's specification
type NodeSpec[T state.GraphState[T]] struct {
	Name        string
	Function    func(context.Context, T, types.Config[T]) (types.NodeResponse[T], error)
	Metadata    map[string]any
	RetryPolicy *RetryPolicy
}

// RetryPolicy defines how a node should handle failures
type RetryPolicy struct {
	MaxAttempts int
	Delay       int // delay in seconds between attempts
}

// Edge represents a connection between nodes
type Edge struct {
	From     string
	To       string
	Metadata map[string]any
}

// Branch represents a conditional branch in the graph
type Branch[T state.GraphState[T]] struct {
	Path     func(context.Context, T, types.Config[T]) string
	Then     string
	Metadata map[string]any
}

// Graph represents the base graph structure
type Graph[T state.GraphState[T]] struct {
	graphID  string
	nodes    map[string]NodeSpec[T]
	edges    []Edge
	adjList  map[string][]Edge // from node -> its edges
	branches map[string][]Branch[T]
	channels map[string]types.Channel[T]

	entryPoint string
	compiled   bool
}

type Option[T state.GraphState[T]] func(*Graph[T])

func WithGraphID[T state.GraphState[T]](id string) Option[T] {
	return func(g *Graph[T]) {
		g.graphID = id
	}
}

// NewGraph creates a new graph instance
func NewGraph[T state.GraphState[T]](name string, opt ...Option[T]) *Graph[T] {
	graphName := defaultGraphName
	if name != "" {
		graphName = name
	}

	g := Graph[T]{
		graphID:  uuid.New().String(),
		nodes:    make(map[string]NodeSpec[T]),
		branches: make(map[string][]Branch[T]),
		channels: make(map[string]types.Channel[T]),
	}
	for _, o := range opt {
		o(&g)
	}

	// remove spaces
	graphName = strings.ReplaceAll(graphName, " ", "-")
	// prepend graph name to graphID
	g.graphID = fmt.Sprintf("%s-%s", graphName, g.graphID)
	return &g
}

func (g *Graph[T]) reachableFrom(startNode string) map[string]bool {
	visited := make(map[string]bool)
	g.dfs(startNode, visited)
	return visited
}

func (g *Graph[T]) dfs(node string, visited map[string]bool) {
	visited[node] = true
	for _, edge := range g.adjList[node] {
		if !visited[edge.To] {
			g.dfs(edge.To, visited)
		}
	}
}

func (g *Graph[T]) checkNotCompiled() error {
	if g.compiled {
		return errors.New("cannot modify a compiled graph")
	}
	return nil
}

func (g *Graph[T]) checkNodeExists(name string) error {
	if _, exists := g.nodes[name]; !exists {
		return fmt.Errorf("node %s does not exist", name)
	}
	return nil
}

// validateEdgeNodes validates source and target nodes
func (g *Graph[T]) validateEdgeNodes(from string, targets []string) error {
	if from == END {
		return errors.New("cannot add edge from END node")
	}

	// Validate source node exists
	if _, exists := g.nodes[from]; !exists {
		return fmt.Errorf("source node %s does not exist", from)
	}

	// Validate all possible targets exist
	for _, target := range targets {
		if target == START {
			return errors.New("cannot add edge to START node")
		}
		if target != END {
			if _, exists := g.nodes[target]; !exists {
				return fmt.Errorf("target node %s does not exist", target)
			}
		}
	}

	return nil
}

func (g *Graph[T]) buildAdjList() {
	g.adjList = make(map[string][]Edge)
	for _, edge := range g.edges {
		g.adjList[edge.From] = append(g.adjList[edge.From], edge)
	}
}

func (g *Graph[T]) ID() string {
	return g.graphID
}

func (g *Graph[T]) HasNode(name string) bool {
	_, exists := g.nodes[name]
	return exists
}

// AddNode adds a new node to the graph
func (g *Graph[T]) AddNode(name string, fn func(context.Context, T, types.Config[T]) (types.NodeResponse[T], error), metadata map[string]any) error {
	if err := g.checkNotCompiled(); err != nil {
		return err
	}

	if _, exists := g.nodes[name]; exists {
		return fmt.Errorf("node %s already exists", name)
	}

	g.nodes[name] = NodeSpec[T]{
		Name:     name,
		Function: fn,
		Metadata: metadata,
	}

	return nil
}

// AddEdge methods for edge management
func (g *Graph[T]) AddEdge(from, to string, metadata map[string]any) error {
	if err := g.checkNotCompiled(); err != nil {
		return err
	}

	if err := g.validateEdgeNodes(from, []string{to}); err != nil {
		return err
	}
	g.edges = append(g.edges, Edge{
		From:     from,
		To:       to,
		Metadata: metadata,
	})
	g.buildAdjList() // or do an incremental update
	return nil
}

func (g *Graph[T]) addBranchCore(
	from string,
	path func(context.Context, T, types.Config[T]) string,
	then string,
	metadata map[string]any,
) error {
	if err := g.checkNotCompiled(); err != nil {
		return err
	}

	if err := g.checkNodeExists(from); err != nil {
		return errors.Wrapf(err, "invalid branch source %s", from)
	}

	// then must be valid or END
	if then != "" && then != END {
		if err := g.checkNodeExists(then); err != nil {
			return errors.Wrapf(err, "invalid branch target %s", then)
		}
	}

	branch := Branch[T]{Path: path, Then: then, Metadata: metadata}
	g.branches[from] = append(g.branches[from], branch)
	return nil
}

// AddBranch adds a conditional branch from a node
func (g *Graph[T]) AddBranch(
	from string,
	path func(context.Context, T, types.Config[T]) string,
	then string,
	metadata map[string]any,
) error {
	return g.addBranchCore(from, path, then, metadata)
}

// AddChannel adds a state management channel
func (g *Graph[T]) AddChannel(name string, channel types.Channel[T]) error {
	if err := g.checkNotCompiled(); err != nil {
		return err
	}

	if _, exists := g.channels[name]; exists {
		return fmt.Errorf("channel %s already exists", name)
	}

	g.channels[name] = channel
	return nil
}

// AddConditionalEdge adds a conditional edge to the graph
func (g *Graph[T]) AddConditionalEdge(
	from string,
	possibleTargets []string,
	condition func(context.Context, T, types.Config[T]) string,
	metadata map[string]any,
) error {
	// 1) Validate node constraints
	if err := g.validateEdgeNodes(from, possibleTargets); err != nil {
		return err
	}

	// 2) Add edges for each possible target
	for _, target := range possibleTargets {
		if err := g.AddEdge(from, target, metadata); err != nil {
			return errors.Wrapf(err, "failed to add edge: %s -> %s", from, target)
		}
	}

	// 3) Reuse a shared “add branch” method
	branchPath := func(ctx context.Context, st T, cfg types.Config[T]) string {
		next := condition(ctx, st, cfg)
		// Return next only if it’s in possibleTargets
		for _, pt := range possibleTargets {
			if pt == next {
				return next
			}
		}
		return "" // or END if you want a fallback
	}

	return g.addBranchCore(from, branchPath, "", metadata)
}

// SetEntryPoint sets the entry point of the graph
func (g *Graph[T]) SetEntryPoint(name string) error {
	if err := g.checkNotCompiled(); err != nil {
		return err
	}
	// Don’t allow END as an entry point
	if name == END {
		return errors.New("cannot set END as entry point")
	}
	// Must exist
	if err := g.checkNodeExists(name); err != nil {
		return err
	}

	g.entryPoint = name
	return nil
}

func (g *Graph[T]) SetEndPoint(name string) error {
	if err := g.checkNotCompiled(); err != nil {
		return err
	}
	// Don’t allow START as an endpoint
	if name == START {
		return errors.New("cannot set START as endpoint")
	}
	// Must exist
	if err := g.checkNodeExists(name); err != nil {
		return err
	}

	if err := g.AddEdge(name, END, nil); err != nil {
		return err
	}
	return nil
}

func (g *Graph[T]) Validate() error {
	if _, exists := g.nodes[g.entryPoint]; !exists {
		return fmt.Errorf("entry point node %s does not exist", g.entryPoint)
	}

	reachable := g.reachableFrom(g.entryPoint)

	// Check all nodes are reachable
	for node := range g.nodes {
		if !reachable[node] {
			return fmt.Errorf("node %s is unreachable from entry point", node)
		}
	}

	if !reachable[END] {
		return errors.New("no path to END from entry point")
	}
	return nil
}

func (g *Graph[T]) BranchEnable(startNode string) error {
	reachable := g.reachableFrom(startNode)
	if !reachable[END] {
		return errors.New("no path to END from node " + startNode)
	}
	return nil
}

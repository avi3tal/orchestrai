package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/avi3tal/orchestrai/internal/state"
	"github.com/avi3tal/orchestrai/internal/types"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// Constants for special nodes
const (
	START            = "START"
	END              = "END"
	defaultGraphName = "graph"
)

// NodeResponse encapsulates the execution result
type NodeResponse struct {
	State  state.GraphState
	Status types.NodeExecutionStatus
}

// NodeSpec represents a node's specification
type NodeSpec struct {
	Name        string
	Function    func(context.Context, state.GraphState, types.Config) (NodeResponse, error)
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
type Branch struct {
	Path     func(context.Context, state.GraphState, types.Config) string
	Then     string
	Metadata map[string]any
}

// Graph represents the base graph structure
type Graph struct {
	graphID  string
	nodes    map[string]NodeSpec
	edges    []Edge
	branches map[string][]Branch
	channels map[string]types.Channel

	entryPoint string
	compiled   bool
}

type Option func(*Graph)

func WithGraphID(id string) Option {
	return func(g *Graph) {
		g.graphID = id
	}
}

// NewGraph creates a new graph instance
func NewGraph(name string, opt ...Option) *Graph {
	graphName := defaultGraphName
	if name != "" {
		graphName = name
	}

	g := Graph{
		graphID:  uuid.New().String(),
		nodes:    make(map[string]NodeSpec),
		branches: make(map[string][]Branch),
		channels: make(map[string]types.Channel),
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

// AddNode adds a new node to the graph
func (g *Graph) AddNode(
	name string,
	fn func(context.Context, state.GraphState, types.Config) (NodeResponse, error),
	metadata map[string]any,
) error {
	if g.compiled {
		return errors.New("cannot add node to compiled graph")
	}

	if _, exists := g.nodes[name]; exists {
		return fmt.Errorf("node %s already exists", name)
	}

	g.nodes[name] = NodeSpec{
		Name:     name,
		Function: fn,
		Metadata: metadata,
	}

	return nil
}

// AddEdge methods for edge management
func (g *Graph) AddEdge(from, to string, metadata map[string]any) error {
	if g.compiled {
		return errors.New("cannot add edge to compiled graph")
	}

	if err := g.validateEdgeNodes(from, []string{to}); err != nil {
		return err
	}

	g.edges = append(g.edges, Edge{
		From:     from,
		To:       to,
		Metadata: metadata,
	})

	return nil
}

// AddBranch adds a conditional branch from a node
func (g *Graph) AddBranch(
	from string,
	path func(context.Context, state.GraphState, types.Config) string,
	then string,
	metadata map[string]any,
) error {
	if g.compiled {
		return errors.New("cannot add branch to compiled graph")
	}

	// Validate source node
	if _, exists := g.nodes[from]; !exists {
		return fmt.Errorf("source node %s does not exist", from)
	}

	// Validate target node if specified
	if then != "" && then != END {
		if _, exists := g.nodes[then]; !exists {
			return fmt.Errorf("target node %s does not exist", then)
		}
	}

	branch := Branch{
		Path:     path,
		Then:     then,
		Metadata: metadata,
	}

	g.branches[from] = append(g.branches[from], branch)
	return nil
}

// AddChannel adds a state management channel
func (g *Graph) AddChannel(name string, channel types.Channel) error {
	if g.compiled {
		return errors.New("cannot add channel to compiled graph")
	}

	if _, exists := g.channels[name]; exists {
		return fmt.Errorf("channel %s already exists", name)
	}

	g.channels[name] = channel
	return nil
}

// AddConditionalEdge adds a conditional edge to the graph
func (g *Graph) AddConditionalEdge(
	from string,
	possibleTargets []string,
	condition func(context.Context, state.GraphState, types.Config) string,
	metadata map[string]any,
) error {
	// Validate nodes first
	if err := g.validateEdgeNodes(from, possibleTargets); err != nil {
		return err
	}

	for _, target := range possibleTargets {
		if err := g.AddEdge(from, target, metadata); err != nil {
			return errors.Wrapf(err, "failed to add conditional edge target %s", target)
		}
	}

	// Create branch with validated condition
	return g.AddBranch(from,
		func(ctx context.Context, st state.GraphState, cfg types.Config) string {
			next := condition(ctx, st, cfg)
			// Validate target is allowed
			for _, target := range possibleTargets {
				if target == next {
					return next
				}
			}
			return ""
		},
		"", // No then node
		metadata,
	)
}

// validateEdgeNodes validates source and target nodes
func (g *Graph) validateEdgeNodes(from string, targets []string) error {
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

// SetEntryPoint sets the entry point of the graph
func (g *Graph) SetEntryPoint(name string) error {
	if g.compiled {
		return errors.New("cannot set entry point on compiled graph")
	}

	if name == END {
		return errors.New("cannot set END as entry point")
	}

	if _, exists := g.nodes[name]; !exists {
		return fmt.Errorf("node %s does not exist", name)
	}

	g.entryPoint = name
	return nil
}

func (g *Graph) Validate() error {
	if g.entryPoint == "" {
		return errors.New("entry point not set")
	}

	// Check if entry point exists
	if _, exists := g.nodes[g.entryPoint]; !exists {
		return fmt.Errorf("entry point node %s does not exist", g.entryPoint)
	}

	// Use DFS to find reachable nodes
	visited := make(map[string]bool)
	reachable := g.dfs(g.entryPoint, visited)

	// Check all nodes are reachable
	for node := range g.nodes {
		if !reachable[node] {
			return fmt.Errorf("node %s is unreachable from entry point", node)
		}
	}

	// Verify path to END exists
	if !reachable[END] {
		return errors.New("no path to END from entry point")
	}

	return nil
}

func (g *Graph) dfs(node string, visited map[string]bool) map[string]bool {
	visited[node] = true
	reachable := make(map[string]bool)
	reachable[node] = true

	// Follow edges
	for _, edge := range g.edges {
		if edge.From == node && !visited[edge.To] {
			for k, v := range g.dfs(edge.To, visited) {
				reachable[k] = v
			}
		}
	}

	return reachable
}

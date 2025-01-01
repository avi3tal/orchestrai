package dag

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"strings"
)

// Constants for special nodes
const (
	START = "START"
	END   = "END"
)

// NodeExecutionStatus represents the current state of node execution
type NodeExecutionStatus string

const (
	StatusCompleted NodeExecutionStatus = "completed"
	StatusPending   NodeExecutionStatus = "pending" // Waiting for user input
	StatusReady     NodeExecutionStatus = "ready"   // Ready to execute
)

// NodeResponse encapsulates the execution result
type NodeResponse[T GraphState[T]] struct {
	State  T
	Status NodeExecutionStatus
}

// NodeSpec represents a node's specification
type NodeSpec[T GraphState[T]] struct {
	Name        string
	Function    func(context.Context, T, Config[T]) (NodeResponse[T], error)
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
type Branch[T GraphState[T]] struct {
	Path     func(context.Context, T, Config[T]) string
	Then     string
	Metadata map[string]any
}

// Graph represents the base graph structure
type Graph[T GraphState[T]] struct {
	graphID  string
	nodes    map[string]NodeSpec[T]
	edges    []Edge
	branches map[string][]Branch[T]
	channels map[string]Channel[T]

	entryPoint string
	compiled   bool
}

type GraphOption[T GraphState[T]] func(*Graph[T])

func WithGraphID[T GraphState[T]](id string) GraphOption[T] {
	return func(g *Graph[T]) {
		g.graphID = id
	}

}

// NewGraph creates a new graph instance
func NewGraph[T GraphState[T]](name string, opt ...GraphOption[T]) *Graph[T] {
	graphName := "graph"
	if name != "" {
		graphName = name
	}

	g := Graph[T]{
		graphID:  uuid.New().String(),
		nodes:    make(map[string]NodeSpec[T]),
		branches: make(map[string][]Branch[T]),
		channels: make(map[string]Channel[T]),
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
func (g *Graph[T]) AddNode(name string, fn func(context.Context, T, Config[T]) (NodeResponse[T], error), metadata map[string]any) error {
	if g.compiled {
		return fmt.Errorf("cannot add node to compiled graph")
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
	if g.compiled {
		return fmt.Errorf("cannot add edge to compiled graph")
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
func (g *Graph[T]) AddBranch(from string, path func(context.Context, T, Config[T]) string, then string, metadata map[string]any) error {
	if g.compiled {
		return fmt.Errorf("cannot add branch to compiled graph")
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

	branch := Branch[T]{
		Path:     path,
		Then:     then,
		Metadata: metadata,
	}

	g.branches[from] = append(g.branches[from], branch)
	return nil
}

// AddChannel adds a state management channel
func (g *Graph[T]) AddChannel(name string, channel Channel[T]) error {
	if g.compiled {
		return fmt.Errorf("cannot add channel to compiled graph")
	}

	if _, exists := g.channels[name]; exists {
		return fmt.Errorf("channel %s already exists", name)
	}

	g.channels[name] = channel
	return nil
}

// AddConditionalEdge adds a conditional edge to the graph
func (g *Graph[T]) AddConditionalEdge(from string, possibleTargets []string, condition func(context.Context, T, Config[T]) string, metadata map[string]any) error {
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
		func(ctx context.Context, state T, cfg Config[T]) string {
			next := condition(ctx, state, cfg)
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
func (g *Graph[T]) validateEdgeNodes(from string, targets []string) error {
	if from == END {
		return fmt.Errorf("cannot add edge from END node")
	}

	// Validate source node exists
	if _, exists := g.nodes[from]; !exists {
		return fmt.Errorf("source node %s does not exist", from)
	}

	// Validate all possible targets exist
	for _, target := range targets {
		if target == START {
			return fmt.Errorf("cannot add edge to START node")
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
func (g *Graph[T]) SetEntryPoint(name string) error {
	if g.compiled {
		return fmt.Errorf("cannot set entry point on compiled graph")
	}

	if name == END {
		return fmt.Errorf("cannot set END as entry point")
	}

	if _, exists := g.nodes[name]; !exists {
		return fmt.Errorf("node %s does not exist", name)
	}

	g.entryPoint = name
	return nil
}

// Validate Updated validation methods
//func (g *Graph[T]) Validate() error {
//	if g.entryPoint == "" {
//		return fmt.Errorf("entry point not set")
//	}
//
//	// Check if entry point exists
//	if _, exists := g.nodes[g.entryPoint]; !exists {
//		return fmt.Errorf("entry point node %s does not exist", g.entryPoint)
//	}
//
//	// Validate all nodes have a path to END
//	unvisited := make(map[string]bool)
//	for node := range g.nodes {
//		unvisited[node] = true
//	}
//
//	if !g.hasPathToEnd(g.entryPoint, unvisited) {
//		return fmt.Errorf("no path to END from entry point %s", g.entryPoint)
//	}
//
//	// Check for unreachable nodes
//	for node := range unvisited {
//		return fmt.Errorf("node %s is unreachable from entry point", node)
//	}
//
//	return nil
//}

func (g *Graph[T]) Validate() error {
	if g.entryPoint == "" {
		return fmt.Errorf("entry point not set")
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
		return fmt.Errorf("no path to END from entry point")
	}

	return nil
}

func (g *Graph[T]) dfs(node string, visited map[string]bool) map[string]bool {
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

//func (g *Graph[T]) hasPathToEnd(node string, unvisited map[string]bool) bool {
//	if node == END {
//		return true
//	}
//
//	delete(unvisited, node)
//
//	// Check direct edges
//	for _, edge := range g.edges {
//		if edge.From == node {
//			// Copy unvisited for each path to prevent cross-path interference
//			pathUnvisited := make(map[string]bool)
//			for k, v := range unvisited {
//				pathUnvisited[k] = v
//			}
//
//			if g.hasPathToEnd(edge.To, pathUnvisited) {
//				// Update main unvisited map with explored path
//				for k := range unvisited {
//					if !pathUnvisited[k] {
//						delete(unvisited, k)
//					}
//				}
//				return true
//			}
//		}
//	}
//
//	// Return false if no valid path found
//	return false
//}

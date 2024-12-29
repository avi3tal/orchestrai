package graph

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avi3tal/orchestrai/internal/types"
	"github.com/google/uuid"
)

// CompiledGraph represents an immutable, executable version of the graph
type CompiledGraph struct {
	// Core properties
	id      string
	version string

	// Immutable graph structure
	nodes    map[string]types.Node
	edges    map[string][]types.Edge
	branches map[string][]types.Edge

	// Runtime configuration
	checkpointer    types.Checkpointer
	store           types.Store
	interruptBefore []string
	interruptAfter  []string
	debug           bool

	// Execution state
	mu             sync.RWMutex
	executionID    string
	currentStep    int64
	lastCheckpoint types.CheckpointID
}

// Execute runs the graph with the given input state
func (g *CompiledGraph) Execute(ctx context.Context, state types.State) (types.State, error) {
	g.mu.Lock()
	g.executionID = uuid.New().String()
	g.currentStep = 0
	g.mu.Unlock()

	// Validate initial state
	if err := state.Validate(); err != nil {
		return nil, NewExecutionError("Execute", "", fmt.Errorf("invalid initial state: %w", err))
	}

	// Create initial checkpoint
	if err := g.createCheckpoint(ctx, types.START, state); err != nil {
		return nil, err
	}

	currentState := state
	currentNode := types.START

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, NewExecutionError("Execute", currentNode, ctx.Err())
		default:
		}

		// Handle interrupts before execution
		if g.shouldInterruptBefore(currentNode) {
			if err := g.handleInterrupt(ctx, currentNode, "before", currentState); err != nil {
				return nil, err
			}
		}

		// Find and execute next node
		nextNode, newState, err := g.executeStep(ctx, currentNode, currentState)
		if err != nil {
			return nil, err
		}
		currentState = newState

		// Handle interrupts after execution
		if g.shouldInterruptAfter(currentNode) {
			if err := g.handleInterrupt(ctx, currentNode, "after", currentState); err != nil {
				return nil, err
			}
		}

		// Check if we've reached the end
		if nextNode == types.END {
			if err := g.createCheckpoint(ctx, types.END, currentState); err != nil {
				return nil, err
			}
			break
		}

		currentNode = nextNode
	}

	return currentState, nil
}

func (g *CompiledGraph) GetNode(id string) (types.Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.nodes[id]
	return node, exists
}

func (g *CompiledGraph) GetEdges(nodeID string) []types.Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges, exists := g.edges[nodeID]
	if !exists {
		return nil
	}
	return edges
}

// executeStep processes a single node and determines the next node to execute
func (g *CompiledGraph) executeStep(ctx context.Context, nodeID string, state types.State) (string, types.State, error) {
	g.mu.Lock()
	g.currentStep++
	step := g.currentStep
	g.mu.Unlock()

	if g.debug {
		g.logDebug("Executing step %d at node %s", step, nodeID)
	}

	// Execute node logic
	nextNode, newState, err := g.executeNode(ctx, nodeID, state)
	if err != nil {
		return "", nil, err
	}

	// Create checkpoint after successful execution
	if err := g.createCheckpoint(ctx, nodeID, newState); err != nil {
		return "", nil, err
	}

	return nextNode, newState, nil
}

// executeNode handles the execution of a single node
func (g *CompiledGraph) executeNode(ctx context.Context, nodeID string, state types.State) (string, types.State, error) {
	// Special handling for START node
	if nodeID == types.START {
		next, err := g.findNextNode(ctx, nodeID, state)
		return next, state, err
	}

	// Get node implementation
	node, exists := g.nodes[nodeID]
	if !exists {
		return "", nil, NewExecutionError("executeNode", nodeID, ErrNodeNotFound)
	}

	executor, ok := node.(types.NodeExecutor)
	if !ok {
		return "", nil, NewExecutionError("executeNode", nodeID, fmt.Errorf("node does not implement NodeExecutor"))
	}

	// Execute node
	newState, err := executor.Execute(ctx, state)
	if err != nil {
		return "", nil, NewExecutionError("executeNode", nodeID, err)
	}

	// Find next node
	next, err := g.findNextNode(ctx, nodeID, newState)
	if err != nil {
		return "", nil, err
	}

	return next, newState, nil
}

// findNextNode determines the next node to execute based on edges and conditions
func (g *CompiledGraph) findNextNode(ctx context.Context, currentNode string, state types.State) (string, error) {
	// Check conditional branches first
	if branches := g.branches[currentNode]; len(branches) > 0 {
		for _, branch := range branches {
			if branch.Condition != nil {
				nextNode, err := branch.Condition(ctx, state)
				if err != nil {
					return "", NewExecutionError("findNextNode", currentNode, err)
				}
				if nextNode != "" {
					return nextNode, nil
				}
			}
		}
	}

	// Then check direct edges
	if edges := g.edges[currentNode]; len(edges) > 0 {
		// For now, take the first edge. In future we might want to support parallel execution
		return edges[0].To, nil
	}

	return "", NewExecutionError("findNextNode", currentNode, fmt.Errorf("no valid edges from node"))
}

// createCheckpoint creates a new checkpoint for the current execution state
func (g *CompiledGraph) createCheckpoint(ctx context.Context, nodeID string, state types.State) error {
	if g.checkpointer == nil {
		return nil // Checkpointing disabled
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	checkpoint := &types.Checkpoint{
		ID:      types.CheckpointID(uuid.New().String()),
		GraphID: g.id,
		NodeID:  nodeID,
		State:   state,
		Metadata: types.CheckpointMetadata{
			CreatedAt: time.Now().UTC(),
			Step:      g.currentStep,
			Tags:      []string{nodeID, fmt.Sprintf("step_%d", g.currentStep)},
			Custom: map[string]interface{}{
				"execution_id": g.executionID,
				"version":      g.version,
			},
		},
		ParentID: g.lastCheckpoint,
	}

	if err := g.checkpointer.Save(ctx, checkpoint); err != nil {
		return NewExecutionError("createCheckpoint", nodeID, err)
	}

	g.lastCheckpoint = checkpoint.ID
	return nil
}

// shouldInterruptBefore checks if a node should be interrupted before execution
func (g *CompiledGraph) shouldInterruptBefore(nodeID string) bool {
	for _, id := range g.interruptBefore {
		if id == nodeID {
			return true
		}
	}
	return false
}

// shouldInterruptAfter checks if a node should be interrupted after execution
func (g *CompiledGraph) shouldInterruptAfter(nodeID string) bool {
	for _, id := range g.interruptAfter {
		if id == nodeID {
			return true
		}
	}
	return false
}

// handleInterrupt processes an interruption point
func (g *CompiledGraph) handleInterrupt(ctx context.Context, nodeID string, phase string, state types.State) error {
	if g.debug {
		g.logDebug("Handling %s interrupt at node %s", phase, nodeID)
	}

	// Validate current state
	if err := state.Validate(); err != nil {
		return NewExecutionError(
			fmt.Sprintf("interrupt_%s", phase),
			nodeID,
			fmt.Errorf("invalid state during interrupt: %w", err),
		)
	}

	return nil
}

// logDebug logs debug information if debug mode is enabled
func (g *CompiledGraph) logDebug(format string, args ...interface{}) {
	if g.debug {
		fmt.Printf("[DEBUG] [%s] %s\n", g.executionID, fmt.Sprintf(format, args...))
	}
}

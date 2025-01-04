package tests

import (
	"context"
	"testing"
	"time"

	"github.com/avi3tal/orchestrai/internal/graph"
	"github.com/avi3tal/orchestrai/pkg/checkpoints"
	"github.com/avi3tal/orchestrai/pkg/types"
	"github.com/stretchr/testify/require"
)

// PendingLoopState A custom state that tracks whether we are “approved” or not.
// We also track how many times we've looped, etc.
type PendingLoopState struct {
	Approved bool
	Counter  int
}

// Validate Satisfy the GraphState interface
func (s PendingLoopState) Validate() error {
	return nil
}

func (s PendingLoopState) Merge(other PendingLoopState) PendingLoopState {
	if other.Approved {
		s.Approved = true
	}
	if other.Counter > s.Counter {
		s.Counter = other.Counter
	}
	return s
}

// The node that "pends" if not approved.
func pendingOrApprovalNode(_ context.Context, st PendingLoopState, _ types.Config[PendingLoopState]) (types.NodeResponse[PendingLoopState], error) {
	if !st.Approved {
		// Simulate that we are waiting for external approval
		return types.NodeResponse[PendingLoopState]{
			State:  st,
			Status: types.StatusPending,
		}, nil
	}
	// If Approved == true, we proceed
	return types.NodeResponse[PendingLoopState]{
		State:  st,
		Status: types.StatusCompleted,
	}, nil
}

// A loop node that increments the counter each time.
func loopNode(_ context.Context, st PendingLoopState, _ types.Config[PendingLoopState]) (types.NodeResponse[PendingLoopState], error) {
	st.Counter++
	return types.NodeResponse[PendingLoopState]{
		State:  st,
		Status: types.StatusCompleted,
	}, nil
}

func TestComplexPendingLoop(t *testing.T) {
	t.Parallel()
	// Build the graph
	g := graph.NewGraph[PendingLoopState]("PendingLoopGraph")

	// Add nodes
	require.NoError(t, g.AddNode("Start", func(_ context.Context, s PendingLoopState, _ types.Config[PendingLoopState]) (types.NodeResponse[PendingLoopState], error) {
		return types.NodeResponse[PendingLoopState]{
			State:  s,
			Status: types.StatusCompleted,
		}, nil
	}, nil))

	require.NoError(t, g.AddNode("LoopNode", loopNode, nil))
	require.NoError(t, g.AddNode("PendingNode", pendingOrApprovalNode, nil))

	// Edges
	require.NoError(t, g.AddEdge("Start", "LoopNode", nil))
	require.NoError(t, g.AddEdge("LoopNode", "PendingNode", nil))

	// Add a branch on LoopNode:
	// If Counter < 2, go back to LoopNode
	// else go to PendingNode
	loopBranch := func(_ context.Context, s PendingLoopState, _ types.Config[PendingLoopState]) string {
		if s.Counter < 2 {
			return "LoopNode"
		}
		return "PendingNode"
	}
	require.NoError(t, g.AddBranch("LoopNode", loopBranch, "", nil))

	// Finally, link PendingNode -> END
	require.NoError(t, g.AddEdge("PendingNode", graph.END, nil))

	// Set start/end
	require.NoError(t, g.SetEntryPoint("Start"))
	require.NoError(t, g.SetEndPoint("PendingNode"))

	// Compile
	compiled, err := g.Compile(
		// Memory checkpoint store so we can resume
		graph.WithCheckpointStore[PendingLoopState](checkpoints.NewMemoryStore[PendingLoopState]()),
		// Some debug & limits
		graph.WithDebug[PendingLoopState](),
		graph.WithMaxSteps[PendingLoopState](100),
		graph.WithTimeout[PendingLoopState](10),
	)
	require.NoError(t, err, "Failed to compile graph")

	// 1) First run: We haven't approved yet => We'll get stuck at the PendingNode
	initialState := PendingLoopState{Approved: false, Counter: 0}

	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()

	pendingState, err := compiled.Run(ctx1, initialState, graph.WithThreadID[PendingLoopState]("pending-thread"))
	require.NoError(t, err, "First run should not fail, but should end in Pending")
	// We expect the graph to have:
	//   - Visited LoopNode enough times so that Counter=2
	//   - Then arrived at PendingNode and returned Pending
	require.Equal(t, 2, pendingState.Counter, "Counter should be incremented twice before hitting PendingNode")

	// 2) Resume run: Now we “approve” and see if we continue from PendingNode
	resumeState := PendingLoopState{Approved: true, Counter: 2}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	finalState, err := compiled.Run(ctx2, resumeState, graph.WithThreadID[PendingLoopState]("pending-thread"))
	require.NoError(t, err, "Second run should succeed since Approved is true now")

	// We verify that the final state is the same we provided (or with merges).
	// The graph should have completed (PendingNode -> END).
	require.True(t, finalState.Approved, "Should remain approved")
	require.Equal(t, 2, finalState.Counter, "Counter should not keep incrementing; loop is done")
}

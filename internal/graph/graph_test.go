package graph

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/avi3tal/orchestrai/pkg/checkpoints"
	"github.com/avi3tal/orchestrai/pkg/types"
	"github.com/stretchr/testify/require"
)

//---------------------------//
// Mock State Implementation //
//---------------------------//

// MyState is a simple example state that implements state.GraphState[MyState].
// In a real scenario, you might have a more complex data structure.
type MyState map[string]interface{}

func (s MyState) Validate() error {
	// In a real scenario, you might want to validate the state
	return nil
}

func (s MyState) Merge(other MyState) MyState {
	// Naive merge: overwrite keys from other
	merged := make(MyState)
	for k, v := range s {
		merged[k] = v
	}
	for k, v := range other {
		merged[k] = v
	}
	return merged
}

//----------------------//
// Mock Node Functions  //
//----------------------//

// simulateCompiler is a trivial node function that simulates a "compiler" action.
func simulateCompiler(_ context.Context, st MyState, _ types.Config[MyState]) (types.NodeResponse[MyState], error) {
	// "Compile" logic... e.g., set some key in the state
	newState := MyState{"status": "compiled"}
	merged := st.Merge(newState)
	return types.NodeResponse[MyState]{
		State:  merged,
		Status: types.StatusCompleted,
	}, nil
}

// simulateResearcher is a trivial node function that simulates a "research" action.
func simulateResearcher(_ context.Context, st MyState, _ types.Config[MyState]) (types.NodeResponse[MyState], error) {
	newState := MyState{"research": "done"}
	merged := st.Merge(newState)
	return types.NodeResponse[MyState]{
		State:  merged,
		Status: types.StatusCompleted,
	}, nil
}

// simulateTreeOfThoughts is a trivial node function that simulates a "tree of thoughts."
func simulateTreeOfThoughts(_ context.Context, st MyState, _ types.Config[MyState]) (types.NodeResponse[MyState], error) {
	newState := MyState{"thinking": true}
	merged := st.Merge(newState)
	return types.NodeResponse[MyState]{
		State:  merged,
		Status: types.StatusCompleted,
	}, nil
}

// simulateErrorNode always returns an error (useful for negative tests).
func simulateErrorNode(_ context.Context, _ MyState, _ types.Config[MyState]) (types.NodeResponse[MyState], error) {
	return types.NodeResponse[MyState]{}, errors.New("simulated failure")
}

//---------------------------//
// Tests for the Graph Logic //
//---------------------------//

func TestGraphScenarios(t *testing.T) {
	t.Parallel()
	// Reusable subfunction to compile & run a graph with an initial state
	// so that we don't rewrite this each time.
	runGraph := func(t *testing.T, g *Graph[MyState], initial MyState) (MyState, error) {
		t.Helper()

		// Compile the graph
		compiled, err := g.Compile()
		require.NoError(t, err, "Failed to compile graph")

		// Run the graph
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return compiled.Run(ctx, initial)
	}

	t.Run("SimpleLinearGraph", func(t *testing.T) {
		t.Parallel()
		g := NewGraph[MyState]("SimpleLinearGraph")
		// Add nodes
		require.NoError(t, g.AddNode("Compiler", simulateCompiler, nil))
		require.NoError(t, g.AddNode("Researcher", simulateResearcher, nil))

		// Add edges: START -> Compiler -> Researcher -> END
		require.NoError(t, g.AddEdge("Compiler", "Researcher", nil))

		// We designate "Compiler" as the entry, "Researcher" as the end node
		require.NoError(t, g.SetEntryPoint("Compiler"))
		require.NoError(t, g.SetEndPoint("Researcher"))

		// Validate & run
		st, err := runGraph(t, g, MyState{"init": true})
		require.NoError(t, err, "Unexpected error running graph")

		// We expect the state to have "status" = "compiled", "research" = "done"
		require.Equal(t, "compiled", st["status"], "Expected 'status' to be 'compiled'")
		require.Equal(t, "done", st["research"], "Expected 'research' to be 'done'")
	})

	t.Run("BranchingScenario", func(t *testing.T) {
		t.Parallel()
		g := NewGraph[MyState]("BranchingScenario")

		// Add nodes
		require.NoError(t, g.AddNode("Compiler", simulateCompiler, nil))
		require.NoError(t, g.AddNode("Researcher", simulateResearcher, nil))
		require.NoError(t, g.AddNode("TreeOfThoughts", simulateTreeOfThoughts, nil))

		// Branch from "Compiler" -> "Researcher" or -> "TreeOfThoughts"
		// The branch function picks next node based on some state logic
		branchFunc := func(_ context.Context, st MyState, _ types.Config[MyState]) string {
			// If the key "useResearcher" is true, go to "Researcher"; else "TreeOfThoughts"
			if val, ok := st["useResearcher"].(bool); ok && val {
				return "Researcher"
			}
			return "TreeOfThoughts"
		}

		// We add a branch for "Compiler"
		require.NoError(t, g.AddBranch("Compiler", branchFunc, "", nil))

		// Add single static edges for DFS/validation
		require.NoError(t, g.AddEdge("Compiler", "Researcher", nil))
		require.NoError(t, g.AddEdge("Compiler", "TreeOfThoughts", nil))

		// Set entry and end
		require.NoError(t, g.SetEntryPoint("Compiler"))

		// Suppose we want "Researcher" to eventually lead to END
		require.NoError(t, g.SetEndPoint("Researcher"))

		// Also set "TreeOfThoughts" to lead to "Researcher" so that we always reach END
		require.NoError(t, g.AddEdge("TreeOfThoughts", "Researcher", nil))

		// Validate & run with "useResearcher" = false => we go to "TreeOfThoughts" -> "Researcher"
		st, err := runGraph(t, g, MyState{"useResearcher": false})
		require.NoError(t, err, "Unexpected error running graph")

		// Expect "thinking" = true, "research" = "done"
		require.Equal(t, true, st["thinking"], "Expected 'thinking' to be true")

		// Now run with "useResearcher" = true => we go directly to "Researcher"
		st, err = runGraph(t, g, MyState{"useResearcher": true})
		require.NoError(t, err, "Unexpected error running graph")

		// Expect "research" = "done"
		require.Equal(t, "done", st["research"], "Expected 'research' to be 'done'")
	})

	t.Run("ConditionalEdgesScenario", func(t *testing.T) {
		t.Parallel()
		g := NewGraph[MyState]("ConditionalEdges")

		require.NoError(t, g.AddNode("Compiler", simulateCompiler, nil))
		require.NoError(t, g.AddNode("Researcher", simulateResearcher, nil))
		require.NoError(t, g.AddNode("TreeOfThoughts", simulateTreeOfThoughts, nil))

		// possibleTargets is ["Researcher", "TreeOfThoughts"]
		condition := func(_ context.Context, st MyState, _ types.Config[MyState]) string {
			if st["switch"] == "A" {
				return "Researcher"
			}
			return "TreeOfThoughts"
		}

		// This will add edges Compiler->Researcher and Compiler->TreeOfThoughts,
		// plus a branch for picking among them.
		require.NoError(t, g.AddConditionalEdge("Compiler", []string{"Researcher", "TreeOfThoughts"}, condition, nil))

		// Next edges
		require.NoError(t, g.AddEdge("TreeOfThoughts", "Researcher", nil))

		// Set entry/exit
		require.NoError(t, g.SetEntryPoint("Compiler"))
		require.NoError(t, g.SetEndPoint("Researcher"))

		// Test with "switch" = "A"
		st, err := runGraph(t, g, MyState{"switch": "A"})
		require.NoError(t, err, "Unexpected error running graph")

		// Expect "research" = "done"
		require.Equal(t, "done", st["research"], "Expected 'research' to be 'done'")

		// Test with no "switch" => go to TreeOfThoughts
		st, err = runGraph(t, g, MyState{"switch": "Z"})
		require.NoError(t, err, "Unexpected error running graph")

		// Expect "thinking" = true
		require.Equal(t, true, st["thinking"], "Expected 'thinking' to be true")
	})

	t.Run("LoopBackScenario", func(t *testing.T) {
		t.Parallel()
		g := NewGraph[MyState]("LoopBack")

		require.NoError(t, g.AddNode("StartNode", func(_ context.Context, st MyState, _ types.Config[MyState]) (types.NodeResponse[MyState], error) {
			// Just mark that we started
			merged := st.Merge(MyState{"started": true})
			return types.NodeResponse[MyState]{State: merged, Status: types.StatusCompleted}, nil
		}, nil))

		// Node that loops back if a certain counter < 3
		require.NoError(t, g.AddNode("LoopNode", func(_ context.Context, st MyState, _ types.Config[MyState]) (types.NodeResponse[MyState], error) {
			count, _ := st["count"].(int)
			count++
			newSt := st.Merge(MyState{"count": count})
			// If we haven't hit 3, we remain completed, but we might jump back
			return types.NodeResponse[MyState]{State: newSt, Status: types.StatusCompleted}, nil
		}, nil))

		// Edges
		require.NoError(t, g.AddEdge("StartNode", "LoopNode", nil))
		// Loop back from "LoopNode" -> "LoopNode" if count < 3
		loopBranch := func(_ context.Context, st MyState, _ types.Config[MyState]) string {
			if c, _ := st["count"].(int); c < 3 {
				return "LoopNode"
			}
			return "END"
		}
		require.NoError(t, g.AddBranch("LoopNode", loopBranch, "", nil))

		// Set entry and end
		require.NoError(t, g.SetEntryPoint("StartNode"))
		// We want the final node to be "LoopNode" -> END
		require.NoError(t, g.SetEndPoint("LoopNode"))

		// Run
		st, err := runGraph(t, g, MyState{})
		require.NoError(t, err, "Unexpected error running graph")

		// Expect "count" to be 3
		require.Equal(t, 3, st["count"], "Expected 'count' to be 3")
	})

	t.Run("MultiBranchScenario", func(t *testing.T) {
		t.Parallel()
		// This scenario has a node with multiple branches, each leading to different sub-graphs
		g := NewGraph[MyState]("MultiBranch")

		require.NoError(t, g.AddNode("Entry", func(_ context.Context, s MyState, _ types.Config[MyState]) (types.NodeResponse[MyState], error) {
			newS := s.Merge(MyState{"entry": true})
			return types.NodeResponse[MyState]{State: newS, Status: types.StatusCompleted}, nil
		}, nil))
		require.NoError(t, g.AddNode("PathA", func(_ context.Context, s MyState, _ types.Config[MyState]) (types.NodeResponse[MyState], error) {
			newS := s.Merge(MyState{"pathA": true})
			return types.NodeResponse[MyState]{State: newS, Status: types.StatusCompleted}, nil
		}, nil))
		require.NoError(t, g.AddNode("PathB", func(_ context.Context, s MyState, _ types.Config[MyState]) (types.NodeResponse[MyState], error) {
			newS := s.Merge(MyState{"pathB": true})
			return types.NodeResponse[MyState]{State: newS, Status: types.StatusCompleted}, nil
		}, nil))

		// We'll just link PathA -> END, PathB -> END
		require.NoError(t, g.AddEdge("PathA", END, nil))
		require.NoError(t, g.AddEdge("PathB", END, nil))

		// Multi-branch from "Entry"
		multiBranchFunc := func(_ context.Context, st MyState, _ types.Config[MyState]) string {
			if st["pick"] == "A" {
				return "PathA"
			}
			if st["pick"] == "B" {
				return "PathB"
			}
			// default
			return "PathA"
		}
		// We can do a single Branch with multiple edges, or a ConditionalEdge
		require.NoError(t, g.AddConditionalEdge("Entry", []string{"PathA", "PathB"}, multiBranchFunc, nil))

		// Set entry/end
		require.NoError(t, g.SetEntryPoint("Entry"))
		require.NoError(t, g.SetEndPoint("PathA"))
		// NOTE: In practice, you might want a fallback from PathB also to END. We did above.

		// Run pick=A
		sA, err := runGraph(t, g, MyState{"pick": "A"})
		require.NoError(t, err, "Failed to run graph with pick=A")

		// We expect "entry" = true, "pathA" = true
		require.Equal(t, true, sA["entry"], "Expected 'entry' to be true")

		// Run pick=B
		sB, err := runGraph(t, g, MyState{"pick": "B"})
		require.NoError(t, err, "Failed to run graph with pick=B")

		// We expect "entry" = true, "pathB" = true
		require.Equal(t, true, sB["entry"], "Expected 'entry' to be true")
	})

	t.Run("NestedGraphScenario", func(t *testing.T) {
		t.Parallel()
		// Example: A "master" graph that has an "inner" graph node.
		// This is conceptual since your code might not directly support nested graphs
		// unless you store them as a node function. We'll do that here.

		// 1) Build an 'inner' graph that we can call from a node function
		innerGraph := NewGraph[MyState]("InnerGraph")
		require.NoError(t, innerGraph.AddNode("InnerStart", simulateCompiler, nil))
		require.NoError(t, innerGraph.AddNode("InnerMid", simulateResearcher, nil))

		if err := innerGraph.AddEdge("InnerStart", "InnerMid", nil); err != nil {
			t.Fatalf("Failed to add edge InnerStart->InnerMid: %v", err)
		}

		require.NoError(t, innerGraph.SetEntryPoint("InnerStart"))
		require.NoError(t, innerGraph.SetEndPoint("InnerMid"))

		// We'll compile the inner graph so we can call it
		compiledInner, err := innerGraph.Compile()
		if err != nil {
			t.Fatalf("Failed to compile inner graph: %v", err)
		}

		// 2) Build the outer graph
		outerGraph := NewGraph[MyState]("OuterGraph")

		// Node that calls the compiled inner graph
		callInnerGraphNode := func(ctx context.Context, s MyState, _ types.Config[MyState]) (types.NodeResponse[MyState], error) {
			// We simply run the inner graph with the current state
			newState, err := compiledInner.Run(ctx, s) //nolint:govet
			if err != nil {
				return types.NodeResponse[MyState]{}, fmt.Errorf("inner graph failed: %w", err)
			}
			return types.NodeResponse[MyState]{State: newState, Status: types.StatusCompleted}, nil
		}

		// Add nodes to the outer graph
		require.NoError(t, outerGraph.AddNode("PreInner", simulateTreeOfThoughts, nil))
		require.NoError(t, outerGraph.AddNode("CallInnerGraph", callInnerGraphNode, nil))
		// Some final node
		require.NoError(t, outerGraph.AddNode("FinalNode", func(_ context.Context, s MyState, _ types.Config[MyState]) (types.NodeResponse[MyState], error) {
			merged := s.Merge(MyState{"final": true})
			return types.NodeResponse[MyState]{State: merged, Status: types.StatusCompleted}, nil
		}, nil))

		// Edges
		require.NoError(t, outerGraph.AddEdge("PreInner", "CallInnerGraph", nil))
		require.NoError(t, outerGraph.AddEdge("CallInnerGraph", "FinalNode", nil))

		// Set entry/end
		require.NoError(t, outerGraph.SetEntryPoint("PreInner"))
		require.NoError(t, outerGraph.SetEndPoint("FinalNode"))

		// Compile outer graph
		compiledOuter, err := outerGraph.Compile()
		if err != nil {
			t.Fatalf("Failed to compile outer graph: %v", err)
		}

		// Run outer graph
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		finalState, err := compiledOuter.Run(ctx, MyState{"outer": true})
		require.NoError(t, err, "Failed to run outer graph")
		if err != nil {
			t.Fatalf("Failed to run outer graph: %v", err)
		}

		// We expect keys from the inner graph merges: "status" = "compiled", "research" = "done"
		if finalState["thinking"] != true {
			t.Errorf("Expected 'thinking' from PreInner, got %v", finalState)
		}
		if finalState["status"] != "compiled" || finalState["research"] != "done" {
			t.Errorf("Did not get merged inner graph state, got %v", finalState)
		}
		if finalState["final"] != true {
			t.Errorf("Did not run 'FinalNode', got %v", finalState)
		}
	})

	t.Run("GraphWithErrorNode", func(t *testing.T) {
		t.Parallel()
		g := NewGraph[MyState]("GraphWithErrorNode")

		require.NoError(t, g.AddNode("WillFail", simulateErrorNode, nil))
		require.NoError(t, g.SetEntryPoint("WillFail"), "SetEntryPoint failed")
		require.NoError(t, g.SetEndPoint("WillFail"), "SetEndPoint failed")

		// We expect run to fail
		_, err := runGraph(t, g, MyState{})
		require.Error(t, err, "Expected error running graph with error node")
	})

	t.Run("CompileAndModifyGraph", func(t *testing.T) {
		t.Parallel()
		g := NewGraph[MyState]("CompileAndModifyGraph")
		require.NoError(t, g.AddNode("NodeA", simulateCompiler, nil))
		require.NoError(t, g.SetEntryPoint("NodeA"))
		require.NoError(t, g.SetEndPoint("NodeA"))

		// Compile
		_, err := g.Compile()
		require.NoError(t, err, "Failed to compile graph")

		// Now try to modify the graph after compilation, expect an error
		err = g.AddNode("NodeB", simulateResearcher, nil)
		require.Error(t, err, "Expected error adding node to compiled graph, got nil")
	})
}

func simulatePendingNode(_ context.Context, st MyState, _ types.Config[MyState]) (types.NodeResponse[MyState], error) {
	// Return a pending status the first time, completed the second time, etc.
	if st["pending"] == nil {
		// Mark we have set it to pending
		newState := st.Merge(MyState{"pending": true})
		return types.NodeResponse[MyState]{
			State:  newState,
			Status: types.StatusPending,
		}, nil
	}

	// If we come back with pending == true, let's complete this time
	return types.NodeResponse[MyState]{
		State:  st,
		Status: types.StatusCompleted,
	}, nil
}

func TestPendingNodeResume(t *testing.T) {
	t.Parallel()
	g := NewGraph[MyState]("PendingNodeGraph")
	require.NoError(t, g.AddNode("MightPending", simulatePendingNode, nil))
	require.NoError(t, g.SetEntryPoint("MightPending"))
	require.NoError(t, g.SetEndPoint("MightPending"))

	// Use an in-memory checkpointer
	compiled, err := g.Compile(
		WithCheckpointStore(checkpoints.NewMemoryStore[MyState]()),
		WithDebug[MyState](),
	)
	require.NoError(t, err)

	// 1) First run -> expect to stop in a pending state
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	st, err := compiled.Run(ctx, MyState{})
	// We expect no error, but the node is "pending," so run should effectively return.
	require.NoError(t, err)
	require.True(t, st["pending"].(bool), "Expected pending in state") //nolint:forcetypeassert

	// 2) Resume from checkpoint
	// simulate that some external event "resolved" the pending condition
	// so the next run picks up at the same node but no longer returns pending
	st2, err := compiled.Run(ctx, st)
	require.NoError(t, err)
	require.True(t, st2["pending"].(bool), "Still expecting pending=true in final state, but status now completed") //nolint:forcetypeassert
}

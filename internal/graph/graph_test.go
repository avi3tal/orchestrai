package graph

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avi3tal/orchestrai/internal/checkpoints"
	"github.com/avi3tal/orchestrai/internal/types"
	"github.com/stretchr/testify/require"
)

// Test States
type SimpleState struct {
	Value int
}

func (s SimpleState) Validate() error {
	return nil
}

func (s SimpleState) Merge(st SimpleState) SimpleState {
	return SimpleState{
		Value: st.Value,
	}
}

type ComplexState struct {
	Numbers []int
	Text    string
	Done    bool
}

func (s ComplexState) Validate() error {
	if s.Numbers == nil {
		return errors.New("numbers cannot be nil")
	}
	return nil
}

func (s ComplexState) Merge(st ComplexState) ComplexState {
	return ComplexState{
		Numbers: st.Numbers,
		Text:    st.Text,
		Done:    st.Done,
	}
}

// Test basic graph with direct edges
func TestSimpleGraphExecution(t *testing.T) {
	t.Parallel()
	g := NewGraph[SimpleState]("simple-graph")

	// Add nodes that increment the value
	require.NoError(t, g.AddNode("add1", func(_ context.Context, s SimpleState, _ types.Config[SimpleState]) (NodeResponse[SimpleState], error) {
		s.Value++
		return NodeResponse[SimpleState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("add2", func(_ context.Context, s SimpleState, _ types.Config[SimpleState]) (NodeResponse[SimpleState], error) {
		s.Value += 2
		return NodeResponse[SimpleState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	// Add direct edges
	require.NoError(t, g.AddEdge("add1", "add2", nil))
	require.NoError(t, g.AddEdge("add2", END, nil))
	require.NoError(t, g.SetEntryPoint("add1"))

	// Compile and run
	compiled, err := g.Compile(WithDebug[SimpleState]())
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), SimpleState{Value: 0}, WithThreadID[SimpleState]("test-1"))
	require.NoError(t, err)
	require.Equal(t, 3, result.Value) // 0 + 1 + 2
}

func TestConditionalGraphExecution(t *testing.T) {
	t.Parallel()
	g := NewGraph[SimpleState]("conditional-graph")

	// Add nodes
	require.NoError(t, g.AddNode("start", func(_ context.Context, s SimpleState, _ types.Config[SimpleState]) (NodeResponse[SimpleState], error) {
		return NodeResponse[SimpleState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("double", func(_ context.Context, s SimpleState, _ types.Config[SimpleState]) (NodeResponse[SimpleState], error) {
		s.Value *= 2
		return NodeResponse[SimpleState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("triple", func(_ context.Context, s SimpleState, _ types.Config[SimpleState]) (NodeResponse[SimpleState], error) {
		s.Value *= 3
		return NodeResponse[SimpleState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	// Connect all possible paths
	require.NoError(t, g.AddConditionalEdge(
		"start",
		[]string{"double", "triple", END},
		func(_ context.Context, s SimpleState, _ types.Config[SimpleState]) string {
			if s.Value < 0 {
				return END
			}
			if s.Value%2 == 0 {
				return "double"
			}
			return "triple"
		},
		nil,
	))

	// Each processing node should end
	require.NoError(t, g.AddEdge("double", "triple", nil))
	require.NoError(t, g.AddEdge("triple", END, nil))

	require.NoError(t, g.SetEntryPoint("start"))

	testCases := []struct {
		name          string
		initialValue  int
		expectedValue int
	}{
		{"even_path", 2, 12}, // Will go to double
		{"odd_path", 3, 9},   // Will go to triple
		{"end_path", -1, -1}, // Will go to END
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compiled, err := g.Compile(WithDebug[SimpleState]())
			require.NoError(t, err)

			result, err := compiled.Run(context.Background(), SimpleState{Value: tc.initialValue}, WithThreadID[SimpleState]("test-"+tc.name))
			require.NoError(t, err)
			require.Equal(t, tc.expectedValue, result.Value)
		})
	}
}

// Test graph with checkpointing
func TestCheckpointedGraphExecution(t *testing.T) {
	t.Parallel()
	g := NewGraph[ComplexState]("checkpoint-graph")
	store := checkpoints.NewMemoryStore[ComplexState]()

	// Add nodes with artificial delays
	require.NoError(t, g.AddNode("step1", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) (NodeResponse[ComplexState], error) {
		time.Sleep(100 * time.Millisecond)
		s.Text = "Step 1 complete"
		s.Numbers = append(s.Numbers, 1)
		return NodeResponse[ComplexState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("step2", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) (NodeResponse[ComplexState], error) {
		time.Sleep(100 * time.Millisecond)
		s.Text = "Step 2 complete"
		s.Numbers = append(s.Numbers, 2)
		return NodeResponse[ComplexState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddEdge("step1", "step2", nil))
	require.NoError(t, g.AddEdge("step2", END, nil))
	require.NoError(t, g.SetEntryPoint("step1"))

	// First run - should complete normally
	compiled, err := g.Compile(WithCheckpointStore(store), WithDebug[ComplexState]())
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)}, WithThreadID[ComplexState]("test-checkpoint"))
	require.NoError(t, err)
	require.Equal(t, "Step 2 complete", result.Text)
	require.ElementsMatch(t, []int{1, 2}, result.Numbers)

	// Verify checkpoint exists
	saved, err := store.Load(context.Background(), types.CheckpointKey{ThreadID: "test-checkpoint", GraphID: g.graphID})
	require.NoError(t, err)
	require.Equal(t, result, saved.State)
}

func TestBranchWithThenNode(t *testing.T) {
	t.Parallel()
	g := NewGraph[ComplexState]("then-branch")

	require.NoError(t, g.AddNode("start", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) (NodeResponse[ComplexState], error) {
		s.Numbers = append(s.Numbers, 1)
		return NodeResponse[ComplexState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("process", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) (NodeResponse[ComplexState], error) {
		s.Numbers = append(s.Numbers, 2)
		return NodeResponse[ComplexState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("cleanup", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) (NodeResponse[ComplexState], error) {
		s.Numbers = append(s.Numbers, 3)
		return NodeResponse[ComplexState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddEdge("start", "process", nil))
	require.NoError(t, g.AddEdge("start", "cleanup", nil))
	require.NoError(t, g.AddEdge("process", END, nil))
	require.NoError(t, g.AddEdge("cleanup", END, nil))

	require.NoError(t, g.AddBranch("start", func(_ context.Context, _ ComplexState, _ types.Config[ComplexState]) string {
		return "process"
	}, "cleanup", nil))

	require.NoError(t, g.SetEntryPoint("start"))

	compiled, err := g.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)})
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, result.Numbers)
}

func TestMultipleBranches(t *testing.T) {
	t.Parallel()
	g := NewGraph[ComplexState]("multi-branch")

	require.NoError(t, g.AddNode("start", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) (NodeResponse[ComplexState], error) {
		return NodeResponse[ComplexState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("pathA", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) (NodeResponse[ComplexState], error) {
		s.Numbers = append(s.Numbers, 1)
		return NodeResponse[ComplexState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("pathB", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) (NodeResponse[ComplexState], error) {
		s.Numbers = append(s.Numbers, 2)
		return NodeResponse[ComplexState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddEdge("start", "pathA", nil))
	require.NoError(t, g.AddEdge("start", "pathB", nil))
	require.NoError(t, g.AddEdge("pathA", END, nil))
	require.NoError(t, g.AddEdge("pathB", END, nil))

	// First branch based on configurable
	require.NoError(t, g.AddBranch("start", func(_ context.Context, _ ComplexState, c types.Config[ComplexState]) string {
		if c.Configurable["path"] == "A" {
			return "pathA"
		}
		return "pathB"
	}, "", nil))

	// Second branch based on state
	require.NoError(t, g.AddBranch("start", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) string {
		if len(s.Numbers) > 0 {
			return "pathA"
		}
		return "pathB"
	}, "", nil))

	require.NoError(t, g.SetEntryPoint("start"))

	compiled, err := g.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)},
		WithConfigurable[ComplexState](map[string]any{"path": "A"}))
	require.NoError(t, err)
	require.Equal(t, []int{1}, result.Numbers)

	result, err = compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)},
		WithConfigurable[ComplexState](map[string]any{"path": "B"}))
	require.NoError(t, err)
	require.Equal(t, []int{2}, result.Numbers)
}

// Test graph with branches
func TestBranchGraphExecution(t *testing.T) {
	t.Parallel()
	g := NewGraph[ComplexState]("branch-graph")

	// Add nodes
	require.NoError(t, g.AddNode("start", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) (NodeResponse[ComplexState], error) {
		s.Numbers = append(s.Numbers, 0)
		return NodeResponse[ComplexState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("process", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) (NodeResponse[ComplexState], error) {
		s.Numbers = append(s.Numbers, len(s.Numbers))
		return NodeResponse[ComplexState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("finalize", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) (NodeResponse[ComplexState], error) {
		s.Done = true
		return NodeResponse[ComplexState]{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	// Add edges for all paths
	require.NoError(t, g.AddEdge("start", "process", nil))
	require.NoError(t, g.AddEdge("process", "process", nil)) // Self loop
	require.NoError(t, g.AddEdge("process", "finalize", nil))
	require.NoError(t, g.AddEdge("finalize", END, nil))

	// Add branch logic
	require.NoError(t, g.AddBranch("process", func(_ context.Context, s ComplexState, _ types.Config[ComplexState]) string {
		if len(s.Numbers) < 3 {
			return "process"
		}
		return "finalize"
	}, "", nil))

	require.NoError(t, g.SetEntryPoint("start"))

	// Compile and run
	compiled, err := g.Compile(WithDebug[ComplexState]())
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)}, WithThreadID[ComplexState]("test-branches"))
	require.NoError(t, err)
	require.True(t, result.Done)
	require.Equal(t, []int{0, 1, 2}, result.Numbers) // Should process 3 times
}

type IntState struct {
	Value int
}

func (s IntState) Validate() error {
	return nil
}

func (s IntState) Merge(st IntState) IntState {
	return IntState{
		Value: st.Value,
	}
}

// Test adding nodes to the graph
func TestAddNode(t *testing.T) {
	t.Parallel()
	graph := NewGraph[IntState]("test-graph")

	// Add valid node
	err := graph.AddNode("node1", func(_ context.Context, st IntState, _ types.Config[IntState]) (NodeResponse[IntState], error) {
		st.Value++
		return NodeResponse[IntState]{State: st, Status: types.StatusCompleted}, nil
	}, nil)
	require.NoError(t, err)

	// Try adding duplicate node
	err = graph.AddNode("node1", nil, nil)
	require.Error(t, err)
}

// Test adding edges to the graph
func TestAddEdge(t *testing.T) {
	t.Parallel()
	graph := NewGraph[IntState]("test-graph")
	_ = graph.AddNode("node1", nil, nil)
	_ = graph.AddNode("node2", nil, nil)

	// Add valid edge
	err := graph.AddEdge("node1", "node2", nil)
	require.NoError(t, err)

	// Add edge with missing nodes
	err = graph.AddEdge("node1", "node3", nil)
	require.Error(t, err)
}

type SimpleStateStr struct {
	Value string
}

func (s SimpleStateStr) Validate() error { return nil }
func (s SimpleStateStr) Merge(other SimpleStateStr) SimpleStateStr {
	if other.Value != "" {
		s.Value = other.Value
	}
	return s
}

func TestGraphBasicFlow(t *testing.T) {
	t.Parallel()
	g := NewGraph[SimpleStateStr]("test")

	err := g.AddNode("node1", func(_ context.Context, _ SimpleStateStr, _ types.Config[SimpleStateStr]) (NodeResponse[SimpleStateStr], error) {
		return NodeResponse[SimpleStateStr]{
			State:  SimpleStateStr{Value: "node1"},
			Status: types.StatusCompleted,
		}, nil
	}, nil)
	require.NoError(t, err)

	err = g.AddNode("node2", func(_ context.Context, _ SimpleStateStr, _ types.Config[SimpleStateStr]) (NodeResponse[SimpleStateStr], error) {
		return NodeResponse[SimpleStateStr]{
			State:  SimpleStateStr{Value: "node2"},
			Status: types.StatusCompleted,
		}, nil
	}, nil)
	require.NoError(t, err)

	err = g.AddEdge("node1", "node2", nil)
	require.NoError(t, err)

	err = g.AddEdge("node2", END, nil)
	require.NoError(t, err)

	err = g.SetEntryPoint("node1")
	require.NoError(t, err)

	compiled, err := g.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), SimpleStateStr{})
	require.NoError(t, err)
	require.Equal(t, "node2", result.Value)
}

func TestGraphCheckpointing(t *testing.T) {
	t.Parallel()
	g := NewGraph[SimpleStateStr]("test")
	store := checkpoints.NewMemoryStore[SimpleStateStr]()

	err := g.AddNode("node1", func(_ context.Context, _ SimpleStateStr, _ types.Config[SimpleStateStr]) (NodeResponse[SimpleStateStr], error) {
		return NodeResponse[SimpleStateStr]{
			State:  SimpleStateStr{Value: "checkpoint1"},
			Status: types.StatusPending,
		}, nil
	}, nil)
	require.NoError(t, err)

	err = g.AddEdge("node1", END, nil)
	require.NoError(t, err)

	err = g.SetEntryPoint("node1")
	require.NoError(t, err)

	compiled, err := g.Compile(WithCheckpointStore[SimpleStateStr](store))
	require.NoError(t, err)

	threadID := "test-thread"
	result, err := compiled.Run(context.Background(), SimpleStateStr{}, WithThreadID[SimpleStateStr](threadID))
	require.NoError(t, err)
	require.Equal(t, "checkpoint1", result.Value)

	checkpoint, err := store.Load(context.Background(), types.CheckpointKey{
		GraphID:  compiled.config.GraphID,
		ThreadID: threadID,
	})
	require.NoError(t, err)
	require.Equal(t, "checkpoint1", checkpoint.State.Value)
}

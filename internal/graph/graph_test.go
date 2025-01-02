package graph

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/avi3tal/orchestrai/internal/checkpoints"
	"github.com/avi3tal/orchestrai/internal/state"
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

func (s SimpleState) Merge(other state.GraphState) state.GraphState {
	o, ok := other.(SimpleState)
	if !ok {
		return s
	}
	return SimpleState{
		Value: o.Value,
	}
}

func (s SimpleState) Clone() state.GraphState {
	return SimpleState{
		Value: s.Value,
	}
}

func (s SimpleState) Dump() ([]byte, error) {
	return []byte(strconv.Itoa(s.Value)), nil
}

func (s SimpleState) Load(data []byte) (state.GraphState, error) {
	return SimpleState{Value: int(data[0])}, nil
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

func (s ComplexState) Merge(other state.GraphState) state.GraphState {
	o, ok := other.(ComplexState)
	if !ok {
		return s
	}
	return ComplexState{
		Numbers: o.Numbers,
		Text:    o.Text,
		Done:    o.Done,
	}
}

func (s ComplexState) Clone() state.GraphState {
	return ComplexState{
		Numbers: append([]int{}, s.Numbers...),
		Text:    s.Text,
		Done:    s.Done,
	}
}

func (s ComplexState) Dump() ([]byte, error) {
	return json.Marshal(s)
}

func (s ComplexState) Load(data []byte) (state.GraphState, error) {
	var st ComplexState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return st, nil
}

type IntState struct {
	Value int
}

func (s IntState) Validate() error {
	return nil
}

func (s IntState) Merge(other state.GraphState) state.GraphState {
	o, _ := other.(IntState)
	return IntState{
		Value: o.Value,
	}
}

func (s IntState) Clone() state.GraphState {
	return IntState{
		Value: s.Value,
	}
}

func (s IntState) Dump() ([]byte, error) {
	return json.Marshal(s)
}

func (s IntState) Load(data []byte) (state.GraphState, error) {
	var st IntState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return st, nil
}

type SimpleStateStr struct {
	Value string
}

func (s SimpleStateStr) Validate() error { return nil }
func (s SimpleStateStr) Merge(other state.GraphState) state.GraphState {
	o, _ := other.(SimpleStateStr)
	if o.Value != "" {
		s.Value = o.Value
	}
	return s
}

func (s SimpleStateStr) Clone() state.GraphState {
	return SimpleStateStr{Value: s.Value}
}

func (s SimpleStateStr) Dump() ([]byte, error) {
	return json.Marshal(s)
}

func (s SimpleStateStr) Load(data []byte) (state.GraphState, error) {
	var st SimpleStateStr
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return st, nil
}

// Test basic graph with direct edges
func TestSimpleGraphExecution(t *testing.T) {
	t.Parallel()
	g := NewGraph("simple-graph")

	// Add nodes that increment the value
	require.NoError(t, g.AddNode("add1", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, ok := other.(SimpleState)
		if !ok {
			return NodeResponse{}, errors.New("invalid state type")
		}
		s.Value++
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("add2", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, ok := other.(SimpleState)
		if !ok {
			return NodeResponse{}, errors.New("invalid state type")
		}
		s.Value += 2
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	// Add direct edges
	require.NoError(t, g.AddEdge("add1", "add2", nil))
	require.NoError(t, g.AddEdge("add2", END, nil))
	require.NoError(t, g.SetEntryPoint("add1"))

	// Compile and run
	compiled, err := g.Compile(WithDebug())
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), SimpleState{Value: 0}, WithThreadID("test-1"))
	require.NoError(t, err)
	r, ok := result.(SimpleState)
	require.True(t, ok)
	require.Equal(t, 3, r.Value) // 0 + 1 + 2
}

func TestConditionalGraphExecution(t *testing.T) {
	t.Parallel()
	g := NewGraph("conditional-graph")

	// Add nodes
	require.NoError(t, g.AddNode("start", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		return NodeResponse{State: other, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("double", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, ok := other.(SimpleState)
		if !ok {
			return NodeResponse{}, errors.New("invalid state type")
		}
		s.Value *= 2
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("triple", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, ok := other.(SimpleState)
		if !ok {
			return NodeResponse{}, errors.New("invalid state type")
		}
		s.Value *= 3
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	// Connect all possible paths
	require.NoError(t, g.AddConditionalEdge(
		"start",
		[]string{"double", "triple", END},
		func(_ context.Context, other state.GraphState, _ types.Config) string {
			s, ok := other.(SimpleState)
			if !ok {
				return END
			}
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
			compiled, err := g.Compile(WithDebug())
			require.NoError(t, err)

			result, err := compiled.Run(context.Background(), SimpleState{Value: tc.initialValue}, WithThreadID("test-"+tc.name))
			require.NoError(t, err)
			r, ok := result.(SimpleState)
			require.True(t, ok)
			require.Equal(t, tc.expectedValue, r.Value)
		})
	}
}

// Test graph with checkpointing
func TestCheckpointedGraphExecution(t *testing.T) {
	t.Parallel()
	g := NewGraph("checkpoint-graph")
	store := checkpoints.NewMemoryStore()

	// Add nodes with artificial delays
	require.NoError(t, g.AddNode("step1", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		time.Sleep(100 * time.Millisecond)
		s, ok := other.(ComplexState)
		if !ok {
			return NodeResponse{}, errors.New("invalid state type")
		}
		s.Text = "Step 1 complete"
		s.Numbers = append(s.Numbers, 1)
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("step2", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, ok := other.(ComplexState)
		if !ok {
			return NodeResponse{}, errors.New("invalid state type")
		}
		time.Sleep(100 * time.Millisecond)
		s.Text = "Step 2 complete"
		s.Numbers = append(s.Numbers, 2)
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddEdge("step1", "step2", nil))
	require.NoError(t, g.AddEdge("step2", END, nil))
	require.NoError(t, g.SetEntryPoint("step1"))

	// First run - should complete normally
	compiled, err := g.Compile(WithCheckpointStore(store), WithDebug())
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)}, WithThreadID("test-checkpoint"))
	require.NoError(t, err)
	r, ok := result.(ComplexState)
	require.True(t, ok)
	require.Equal(t, "Step 2 complete", r.Text)
	require.ElementsMatch(t, []int{1, 2}, r.Numbers)

	// Verify checkpoint exists
	saved, err := store.Load(context.Background(), types.CheckpointKey{ThreadID: "test-checkpoint", GraphID: g.graphID})
	require.NoError(t, err)
	require.Equal(t, result, saved.State)
}

func TestBranchWithThenNode(t *testing.T) {
	t.Parallel()
	g := NewGraph("then-branch")

	require.NoError(t, g.AddNode("start", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, ok := other.(ComplexState)
		if !ok {
			return NodeResponse{}, errors.New("invalid state type")
		}
		s.Numbers = append(s.Numbers, 1)
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("process", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, ok := other.(ComplexState)
		if !ok {
			return NodeResponse{}, errors.New("invalid state type")
		}
		s.Numbers = append(s.Numbers, 2)
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("cleanup", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, ok := other.(ComplexState)
		if !ok {
			return NodeResponse{}, errors.New("invalid state type")
		}
		s.Numbers = append(s.Numbers, 3)
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddEdge("start", "process", nil))
	require.NoError(t, g.AddEdge("start", "cleanup", nil))
	require.NoError(t, g.AddEdge("process", END, nil))
	require.NoError(t, g.AddEdge("cleanup", END, nil))

	require.NoError(t, g.AddBranch("start", func(_ context.Context, _ state.GraphState, _ types.Config) string {
		return "process"
	}, "cleanup", nil))

	require.NoError(t, g.SetEntryPoint("start"))

	compiled, err := g.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)})
	require.NoError(t, err)
	r, ok := result.(ComplexState)
	require.True(t, ok)
	require.Equal(t, []int{1, 2, 3}, r.Numbers)
}

func TestMultipleBranches(t *testing.T) {
	t.Parallel()
	g := NewGraph("multi-branch")

	require.NoError(t, g.AddNode("start", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, ok := other.(ComplexState)
		if !ok {
			return NodeResponse{}, errors.New("invalid state type")
		}
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("pathA", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, ok := other.(ComplexState)
		if !ok {
			return NodeResponse{}, errors.New("invalid state type")
		}
		s.Numbers = append(s.Numbers, 1)
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("pathB", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, ok := other.(ComplexState)
		if !ok {
			return NodeResponse{}, errors.New("invalid state type")
		}
		s.Numbers = append(s.Numbers, 2)
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddEdge("start", "pathA", nil))
	require.NoError(t, g.AddEdge("start", "pathB", nil))
	require.NoError(t, g.AddEdge("pathA", END, nil))
	require.NoError(t, g.AddEdge("pathB", END, nil))

	// First branch based on configurable
	require.NoError(t, g.AddBranch("start", func(_ context.Context, _ state.GraphState, c types.Config) string {
		if c.Configurable["path"] == "A" {
			return "pathA"
		}
		return "pathB"
	}, "", nil))

	// Second branch based on state
	require.NoError(t, g.AddBranch("start", func(_ context.Context, other state.GraphState, _ types.Config) string {
		s, _ := other.(ComplexState)
		if len(s.Numbers) > 0 {
			return "pathA"
		}
		return "pathB"
	}, "", nil))

	require.NoError(t, g.SetEntryPoint("start"))

	compiled, err := g.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)},
		WithConfigurable(map[string]any{"path": "A"}))
	require.NoError(t, err)
	r, ok := result.(ComplexState)
	require.True(t, ok)
	require.Equal(t, []int{1}, r.Numbers)

	result, err = compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)},
		WithConfigurable(map[string]any{"path": "B"}))
	require.NoError(t, err)
	r, ok = result.(ComplexState)
	require.True(t, ok)
	require.Equal(t, []int{2}, r.Numbers)
}

// Test graph with branches
func TestBranchGraphExecution(t *testing.T) {
	t.Parallel()
	g := NewGraph("branch-graph")

	// Add nodes
	require.NoError(t, g.AddNode("start", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, _ := other.(ComplexState)
		s.Numbers = append(s.Numbers, 0)
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("process", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, _ := other.(ComplexState)
		s.Numbers = append(s.Numbers, len(s.Numbers))
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("finalize", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, _ := other.(ComplexState)
		s.Done = true
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil))

	// Add edges for all paths
	require.NoError(t, g.AddEdge("start", "process", nil))
	require.NoError(t, g.AddEdge("process", "process", nil)) // Self loop
	require.NoError(t, g.AddEdge("process", "finalize", nil))
	require.NoError(t, g.AddEdge("finalize", END, nil))

	// Add branch logic
	require.NoError(t, g.AddBranch("process", func(_ context.Context, other state.GraphState, _ types.Config) string {
		s, _ := other.(ComplexState)
		if len(s.Numbers) < 3 {
			return "process"
		}
		return "finalize"
	}, "", nil))

	require.NoError(t, g.SetEntryPoint("start"))

	// Compile and run
	compiled, err := g.Compile(WithDebug())
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)}, WithThreadID("test-branches"))
	require.NoError(t, err)
	r, ok := result.(ComplexState)
	require.True(t, ok)
	require.True(t, r.Done)
	require.Equal(t, []int{0, 1, 2}, r.Numbers) // Should process 3 times
}

// Test adding nodes to the graph
func TestAddNode(t *testing.T) {
	t.Parallel()
	graph := NewGraph("test-graph")

	// Add valid node
	err := graph.AddNode("node1", func(_ context.Context, other state.GraphState, _ types.Config) (NodeResponse, error) {
		s, _ := other.(IntState)
		s.Value++
		return NodeResponse{State: s, Status: types.StatusCompleted}, nil
	}, nil)
	require.NoError(t, err)

	// Try adding duplicate node
	err = graph.AddNode("node1", nil, nil)
	require.Error(t, err)
}

// Test adding edges to the graph
func TestAddEdge(t *testing.T) {
	t.Parallel()
	graph := NewGraph("test-graph")
	_ = graph.AddNode("node1", nil, nil)
	_ = graph.AddNode("node2", nil, nil)

	// Add valid edge
	err := graph.AddEdge("node1", "node2", nil)
	require.NoError(t, err)

	// Add edge with missing nodes
	err = graph.AddEdge("node1", "node3", nil)
	require.Error(t, err)
}

func TestGraphBasicFlow(t *testing.T) {
	t.Parallel()
	g := NewGraph("test")

	err := g.AddNode("node1", func(_ context.Context, _ state.GraphState, _ types.Config) (NodeResponse, error) {
		return NodeResponse{
			State:  SimpleStateStr{Value: "node1"},
			Status: types.StatusCompleted,
		}, nil
	}, nil)
	require.NoError(t, err)

	err = g.AddNode("node2", func(_ context.Context, _ state.GraphState, _ types.Config) (NodeResponse, error) {
		return NodeResponse{
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
	r, ok := result.(SimpleStateStr)
	require.True(t, ok)
	require.Equal(t, "node2", r.Value)
}

func TestGraphCheckpointing(t *testing.T) {
	t.Parallel()
	g := NewGraph("test")
	store := checkpoints.NewMemoryStore()

	err := g.AddNode("node1", func(_ context.Context, _ state.GraphState, _ types.Config) (NodeResponse, error) {
		return NodeResponse{
			State:  SimpleStateStr{Value: "checkpoint1"},
			Status: types.StatusPending,
		}, nil
	}, nil)
	require.NoError(t, err)

	err = g.AddEdge("node1", END, nil)
	require.NoError(t, err)

	err = g.SetEntryPoint("node1")
	require.NoError(t, err)

	compiled, err := g.Compile(WithCheckpointStore(store))
	require.NoError(t, err)

	threadID := "test-thread"
	result, err := compiled.Run(context.Background(), SimpleStateStr{}, WithThreadID(threadID))
	require.NoError(t, err)
	r, ok := result.(SimpleStateStr)
	require.True(t, ok)
	require.Equal(t, "checkpoint1", r.Value)

	checkpoint, err := store.Load(context.Background(), types.CheckpointKey{
		GraphID:  compiled.config.GraphID,
		ThreadID: threadID,
	})
	require.NoError(t, err)
	st, ok := checkpoint.State.(SimpleStateStr)
	require.True(t, ok)
	require.Equal(t, "checkpoint1", st.Value)
}

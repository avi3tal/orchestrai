package dag

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
		return fmt.Errorf("numbers cannot be nil")
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
	g := NewGraph[SimpleState]("simple-graph")

	// Add nodes that increment the value
	require.NoError(t, g.AddNode("add1", func(ctx context.Context, s SimpleState, c Config[SimpleState]) (NodeResponse[SimpleState], error) {
		s.Value++
		return NodeResponse[SimpleState]{State: s, Status: StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("add2", func(ctx context.Context, s SimpleState, c Config[SimpleState]) (NodeResponse[SimpleState], error) {
		s.Value += 2
		return NodeResponse[SimpleState]{State: s, Status: StatusCompleted}, nil
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
	assert.Equal(t, 3, result.Value) // 0 + 1 + 2
}

func TestConditionalGraphExecution(t *testing.T) {
	g := NewGraph[SimpleState]("conditional-graph")

	// Add nodes
	require.NoError(t, g.AddNode("start", func(ctx context.Context, s SimpleState, c Config[SimpleState]) (NodeResponse[SimpleState], error) {
		return NodeResponse[SimpleState]{State: s, Status: StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("double", func(ctx context.Context, s SimpleState, c Config[SimpleState]) (NodeResponse[SimpleState], error) {
		s.Value *= 2
		return NodeResponse[SimpleState]{State: s, Status: StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("triple", func(ctx context.Context, s SimpleState, c Config[SimpleState]) (NodeResponse[SimpleState], error) {
		s.Value *= 3
		return NodeResponse[SimpleState]{State: s, Status: StatusCompleted}, nil
	}, nil))

	// Connect all possible paths
	require.NoError(t, g.AddConditionalEdge(
		"start",
		[]string{"double", "triple", END},
		func(ctx context.Context, s SimpleState, c Config[SimpleState]) string {
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
			compiled, err := g.Compile(WithDebug[SimpleState]())
			require.NoError(t, err)

			result, err := compiled.Run(context.Background(), SimpleState{Value: tc.initialValue}, WithThreadID[SimpleState](fmt.Sprintf("test-%s", tc.name)))
			require.NoError(t, err)
			assert.Equal(t, tc.expectedValue, result.Value)
		})
	}
}

// Test graph with channels
func TestChannelGraphExecution(t *testing.T) {
	g := NewGraph[ComplexState]("channel-graph")

	// Add channels
	require.NoError(t, g.AddChannel("numbers", LastValueChannelType))

	// Add nodes
	require.NoError(t, g.AddNode("start", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (NodeResponse[ComplexState], error) {
		return NodeResponse[ComplexState]{State: s, Status: StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("producer1", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (NodeResponse[ComplexState], error) {
		s.Numbers = append(s.Numbers, 1, 2, 3)
		channel := g.channels["numbers"]
		err := channel.Write(ctx, s, c)
		return NodeResponse[ComplexState]{State: s, Status: StatusCompleted}, err
	}, nil))

	require.NoError(t, g.AddNode("producer2", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (NodeResponse[ComplexState], error) {
		// Read previous state from channel
		channel := g.channels["numbers"]
		prevState, err := channel.Read(ctx, c)
		if err != nil {
			return NodeResponse[ComplexState]{State: s, Status: StatusCompleted}, err
		}

		// Append new numbers to existing ones
		s.Numbers = append(prevState.Numbers, 4, 5, 6)
		err = channel.Write(ctx, s, c)
		return NodeResponse[ComplexState]{State: s, Status: StatusCompleted}, err
	}, nil))

	require.NoError(t, g.AddNode("consumer", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (NodeResponse[ComplexState], error) {
		channel := g.channels["numbers"]
		state, err := channel.Read(ctx, c)
		if err != nil {
			return NodeResponse[ComplexState]{State: s, Status: StatusCompleted}, err
		}
		s.Numbers = state.Numbers
		s.Done = true
		return NodeResponse[ComplexState]{State: s, Status: StatusCompleted}, nil
	}, nil))

	// Add sequential path
	require.NoError(t, g.AddEdge("start", "producer1", nil))
	require.NoError(t, g.AddEdge("producer1", "producer2", nil))
	require.NoError(t, g.AddEdge("producer2", "consumer", nil))
	require.NoError(t, g.AddEdge("consumer", END, nil))

	require.NoError(t, g.SetEntryPoint("start"))

	compiled, err := g.Compile(WithDebug[ComplexState]())
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)}, WithThreadID[ComplexState]("test-channels"))
	require.NoError(t, err)
	assert.True(t, result.Done)
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6}, result.Numbers)
}

// Test graph with checkpointing
func TestCheckpointedGraphExecution(t *testing.T) {
	g := NewGraph[ComplexState]("checkpoint-graph")
	store := NewMemoryStore[ComplexState]()

	// Add nodes with artificial delays
	require.NoError(t, g.AddNode("step1", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (NodeResponse[ComplexState], error) {
		time.Sleep(100 * time.Millisecond)
		s.Text = "Step 1 complete"
		s.Numbers = append(s.Numbers, 1)
		return NodeResponse[ComplexState]{State: s, Status: StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("step2", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (NodeResponse[ComplexState], error) {
		time.Sleep(100 * time.Millisecond)
		s.Text = "Step 2 complete"
		s.Numbers = append(s.Numbers, 2)
		return NodeResponse[ComplexState]{State: s, Status: StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddEdge("step1", "step2", nil))
	require.NoError(t, g.AddEdge("step2", END, nil))
	require.NoError(t, g.SetEntryPoint("step1"))

	// First run - should complete normally
	compiled, err := g.Compile(WithCheckpointStore(store), WithDebug[ComplexState]())
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)}, WithThreadID[ComplexState]("test-checkpoint"))
	require.NoError(t, err)
	assert.Equal(t, "Step 2 complete", result.Text)
	assert.ElementsMatch(t, []int{1, 2}, result.Numbers)

	// Verify checkpoint exists
	saved, err := store.Load(context.Background(), CheckpointKey{ThreadID: "test-checkpoint", GraphID: g.graphID})
	require.NoError(t, err)
	assert.Equal(t, result, saved.State)
}

// Test graph with branches
func TestBranchGraphExecution(t *testing.T) {
	g := NewGraph[ComplexState]("branch-graph")

	// Add nodes
	require.NoError(t, g.AddNode("start", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (NodeResponse[ComplexState], error) {
		s.Numbers = append(s.Numbers, 0)
		return NodeResponse[ComplexState]{State: s, Status: StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("process", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (NodeResponse[ComplexState], error) {
		s.Numbers = append(s.Numbers, len(s.Numbers))
		return NodeResponse[ComplexState]{State: s, Status: StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("finalize", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (NodeResponse[ComplexState], error) {
		s.Done = true
		return NodeResponse[ComplexState]{State: s, Status: StatusCompleted}, nil
	}, nil))

	// Add edges for all paths
	require.NoError(t, g.AddEdge("start", "process", nil))
	require.NoError(t, g.AddEdge("process", "process", nil)) // Self loop
	require.NoError(t, g.AddEdge("process", "finalize", nil))
	require.NoError(t, g.AddEdge("finalize", END, nil))

	// Add branch logic
	require.NoError(t, g.AddBranch("process", func(ctx context.Context, s ComplexState, c Config[ComplexState]) string {
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
	assert.True(t, result.Done)
	assert.Equal(t, []int{0, 1, 2}, result.Numbers) // Should process 3 times
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
	graph := NewGraph[IntState]("test-graph")

	// Add valid node
	err := graph.AddNode("node1", func(ctx context.Context, state IntState, config Config[IntState]) (NodeResponse[IntState], error) {
		state.Value += 1
		return NodeResponse[IntState]{State: state, Status: StatusCompleted}, nil
	}, nil)
	assert.NoError(t, err)

	// Try adding duplicate node
	err = graph.AddNode("node1", nil, nil)
	assert.Error(t, err)
}

// Test adding edges to the graph
func TestAddEdge(t *testing.T) {
	graph := NewGraph[IntState]("test-graph")
	_ = graph.AddNode("node1", nil, nil)
	_ = graph.AddNode("node2", nil, nil)

	// Add valid edge
	err := graph.AddEdge("node1", "node2", nil)
	assert.NoError(t, err)

	// Add edge with missing nodes
	err = graph.AddEdge("node1", "node3", nil)
	assert.Error(t, err)
}

type TestState struct {
	Value int
}

func (s TestState) Validate() error {
	if s.Value < 0 {
		return fmt.Errorf("value cannot be negative")
	}
	return nil
}

func (s TestState) Merge(other TestState) TestState {
	return TestState{Value: s.Value + other.Value}
}

// Test state validation
func TestStateValidation(t *testing.T) {
	state := TestState{Value: 5}
	assert.NoError(t, state.Validate())

	invalidState := TestState{Value: -1}
	assert.Error(t, invalidState.Validate())
}

// Test state merging
func TestStateMerge(t *testing.T) {
	state1 := TestState{Value: 5}
	state2 := TestState{Value: 3}
	mergedState := state1.Merge(state2)
	assert.Equal(t, 8, mergedState.Value)
}

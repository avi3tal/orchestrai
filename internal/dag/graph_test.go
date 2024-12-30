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

// Test basic graph with direct edges
func TestSimpleGraphExecution(t *testing.T) {
	g := NewGraph[SimpleState]()

	// Add nodes that increment the value
	require.NoError(t, g.AddNode("add1", func(ctx context.Context, s SimpleState, c Config[SimpleState]) (SimpleState, error) {
		s.Value++
		return s, nil
	}, nil))

	require.NoError(t, g.AddNode("add2", func(ctx context.Context, s SimpleState, c Config[SimpleState]) (SimpleState, error) {
		s.Value += 2
		return s, nil
	}, nil))

	// Add direct edges
	require.NoError(t, g.AddEdge("add1", "add2", nil))
	require.NoError(t, g.AddEdge("add2", END, nil))
	require.NoError(t, g.SetEntryPoint("add1"))

	// Compile and run
	compiled, err := g.Compile(Config[SimpleState]{
		ThreadID: "test-1",
		Debug:    true,
	})
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), SimpleState{Value: 0})
	require.NoError(t, err)
	assert.Equal(t, 3, result.Value) // 0 + 1 + 2
}

func TestConditionalGraphExecution(t *testing.T) {
	g := NewGraph[SimpleState]()

	// Add nodes
	require.NoError(t, g.AddNode("start", func(ctx context.Context, s SimpleState, c Config[SimpleState]) (SimpleState, error) {
		return s, nil
	}, nil))

	require.NoError(t, g.AddNode("double", func(ctx context.Context, s SimpleState, c Config[SimpleState]) (SimpleState, error) {
		s.Value *= 2
		return s, nil
	}, nil))

	require.NoError(t, g.AddNode("triple", func(ctx context.Context, s SimpleState, c Config[SimpleState]) (SimpleState, error) {
		s.Value *= 3
		return s, nil
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
			compiled, err := g.Compile(Config[SimpleState]{
				ThreadID: fmt.Sprintf("test-%s", tc.name),
				Debug:    true,
			})
			require.NoError(t, err)

			result, err := compiled.Run(context.Background(), SimpleState{Value: tc.initialValue})
			require.NoError(t, err)
			assert.Equal(t, tc.expectedValue, result.Value)
		})
	}
}

// Test graph with channels
func TestChannelGraphExecution(t *testing.T) {
	g := NewGraph[ComplexState]()

	// Add channels
	require.NoError(t, g.AddChannel("numbers", LastValueChannelType))

	// Add nodes
	require.NoError(t, g.AddNode("start", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (ComplexState, error) {
		return s, nil
	}, nil))

	require.NoError(t, g.AddNode("producer1", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (ComplexState, error) {
		s.Numbers = append(s.Numbers, 1, 2, 3)
		channel := g.channels["numbers"]
		err := channel.Write(ctx, s, c)
		return s, err
	}, nil))

	require.NoError(t, g.AddNode("producer2", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (ComplexState, error) {
		// Read previous state from channel
		channel := g.channels["numbers"]
		prevState, err := channel.Read(ctx, c)
		if err != nil {
			return s, err
		}

		// Append new numbers to existing ones
		s.Numbers = append(prevState.Numbers, 4, 5, 6)
		err = channel.Write(ctx, s, c)
		return s, err
	}, nil))

	require.NoError(t, g.AddNode("consumer", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (ComplexState, error) {
		channel := g.channels["numbers"]
		state, err := channel.Read(ctx, c)
		if err != nil {
			return s, err
		}
		s.Numbers = state.Numbers
		s.Done = true
		return s, nil
	}, nil))

	// Add sequential path
	require.NoError(t, g.AddEdge("start", "producer1", nil))
	require.NoError(t, g.AddEdge("producer1", "producer2", nil))
	require.NoError(t, g.AddEdge("producer2", "consumer", nil))
	require.NoError(t, g.AddEdge("consumer", END, nil))

	require.NoError(t, g.SetEntryPoint("start"))

	compiled, err := g.Compile(Config[ComplexState]{
		ThreadID: "test-channels",
		Debug:    true,
	})
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)})
	require.NoError(t, err)
	assert.True(t, result.Done)
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6}, result.Numbers)
}

// Test graph with checkpointing
func TestCheckpointedGraphExecution(t *testing.T) {
	g := NewGraph[ComplexState]()
	checkpointer := NewMemoryCheckpointer[ComplexState]()

	// Add nodes with artificial delays
	require.NoError(t, g.AddNode("step1", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (ComplexState, error) {
		time.Sleep(100 * time.Millisecond)
		s.Text = "Step 1 complete"
		s.Numbers = append(s.Numbers, 1)
		return s, nil
	}, nil))

	require.NoError(t, g.AddNode("step2", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (ComplexState, error) {
		time.Sleep(100 * time.Millisecond)
		s.Text = "Step 2 complete"
		s.Numbers = append(s.Numbers, 2)
		return s, nil
	}, nil))

	require.NoError(t, g.AddEdge("step1", "step2", nil))
	require.NoError(t, g.AddEdge("step2", END, nil))
	require.NoError(t, g.SetEntryPoint("step1"))

	// First run - should complete normally
	compiled, err := g.Compile(Config[ComplexState]{
		ThreadID:     "test-checkpoint",
		Checkpointer: checkpointer,
		Debug:        true,
	})
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)})
	require.NoError(t, err)
	assert.Equal(t, "Step 2 complete", result.Text)
	assert.ElementsMatch(t, []int{1, 2}, result.Numbers)

	// Verify checkpoint exists
	saved, err := checkpointer.Load(context.Background(), Config[ComplexState]{ThreadID: "test-checkpoint"})
	require.NoError(t, err)
	assert.Equal(t, result, saved)
}

// Test graph with branches
func TestBranchGraphExecution(t *testing.T) {
	g := NewGraph[ComplexState]()

	// Add nodes
	require.NoError(t, g.AddNode("start", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (ComplexState, error) {
		s.Numbers = append(s.Numbers, 0)
		return s, nil
	}, nil))

	require.NoError(t, g.AddNode("process", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (ComplexState, error) {
		s.Numbers = append(s.Numbers, len(s.Numbers))
		return s, nil
	}, nil))

	require.NoError(t, g.AddNode("finalize", func(ctx context.Context, s ComplexState, c Config[ComplexState]) (ComplexState, error) {
		s.Done = true
		return s, nil
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

	compiled, err := g.Compile(Config[ComplexState]{
		ThreadID: "test-branches",
		Debug:    true,
	})
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)})
	require.NoError(t, err)
	assert.True(t, result.Done)
	assert.Equal(t, []int{0, 1, 2}, result.Numbers) // Should process 3 times
}

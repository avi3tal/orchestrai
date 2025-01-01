package channels

import (
	"context"
	"fmt"
	"github.com/avi3tal/orchestrai/internal/dag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type TestState struct {
	Value string
}

func (s TestState) Validate() error {
	return nil
}

func (s TestState) Merge(other TestState) TestState {
	if other.Value != "" {
		s.Value = other.Value
	}
	return s
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

func TestLastValueChannel(t *testing.T) {
	ctx := context.Background()
	ch := NewLastValue[TestState]()
	cfg := dag.Config[TestState]{ThreadID: "test"}

	t.Run("Write and Read", func(t *testing.T) {
		state := TestState{Value: "test"}
		err := ch.Write(ctx, state, cfg)
		assert.NoError(t, err)

		got, err := ch.Read(ctx, cfg)
		assert.NoError(t, err)
		assert.Equal(t, state, got)
	})

	t.Run("Multiple Writes", func(t *testing.T) {
		states := []TestState{
			{Value: "first"},
			{Value: "second"},
			{Value: "third"},
		}

		for _, s := range states {
			err := ch.Write(ctx, s, cfg)
			assert.NoError(t, err)
		}

		got, err := ch.Read(ctx, cfg)
		assert.NoError(t, err)
		assert.Equal(t, states[len(states)-1], got)
	})
}

func TestBarrierChannel(t *testing.T) {
	ctx := context.Background()
	required := []string{"node1", "node2"}
	ch := NewBarrierChannel[TestState](required)

	t.Run("Write Before All Required", func(t *testing.T) {
		state := TestState{Value: "test"}
		err := ch.Write(ctx, state, dag.Config[TestState]{ThreadID: "node1"})
		assert.NoError(t, err)

		_, err = ch.Read(ctx, dag.Config[TestState]{})
		assert.Error(t, err)
	})

	t.Run("Write All Required", func(t *testing.T) {
		state1 := TestState{Value: "node1"}
		state2 := TestState{Value: "node2"}

		err := ch.Write(ctx, state1, dag.Config[TestState]{ThreadID: "node1"})
		assert.NoError(t, err)

		err = ch.Write(ctx, state2, dag.Config[TestState]{ThreadID: "node2"})
		assert.NoError(t, err)

		got, err := ch.Read(ctx, dag.Config[TestState]{})
		assert.NoError(t, err)
		assert.Equal(t, state2, got)
	})

	t.Run("Invalid Node Write", func(t *testing.T) {
		err := ch.Write(ctx, TestState{}, dag.Config[TestState]{ThreadID: "invalid"})
		assert.Error(t, err)
	})
}

func TestDynamicBarrierChannel(t *testing.T) {
	ctx := context.Background()
	ch := NewDynamicBarrierChannel[TestState]()

	t.Run("Dynamic Registration", func(t *testing.T) {
		ch.AddRequired("node1")
		ch.AddRequired("node2")

		state := TestState{Value: "test"}
		err := ch.Write(ctx, state, dag.Config[TestState]{ThreadID: "node1"})
		assert.NoError(t, err)

		_, err = ch.Read(ctx, dag.Config[TestState]{})
		assert.Error(t, err)

		err = ch.Write(ctx, state, dag.Config[TestState]{ThreadID: "node2"})
		assert.NoError(t, err)

		got, err := ch.Read(ctx, dag.Config[TestState]{})
		assert.NoError(t, err)
		assert.Equal(t, state, got)
	})

	t.Run("Concurrent Access", func(t *testing.T) {
		ch := NewDynamicBarrierChannel[TestState]()
		nodes := []string{"node1", "node2", "node3"}

		for _, node := range nodes {
			ch.AddRequired(node)
		}

		done := make(chan bool)
		for _, node := range nodes {
			nodeID := node
			go func() {
				err := ch.Write(ctx, TestState{Value: nodeID}, dag.Config[TestState]{ThreadID: nodeID})
				assert.NoError(t, err)
				done <- true
			}()
		}

		for range nodes {
			<-done
		}

		got, err := ch.Read(ctx, dag.Config[TestState]{})
		assert.NoError(t, err)
		assert.NotEmpty(t, got.Value)
	})
}

// Test graph with channels
func TestChannelGraphExecution(t *testing.T) {
	g := dag.NewGraph[ComplexState]("channel-graph")

	numbersChannel := NewLastValue[ComplexState]()
	// Add channels
	require.NoError(t, g.AddChannel("numbers", numbersChannel))

	// Add nodes
	require.NoError(t, g.AddNode("start", func(ctx context.Context, s ComplexState, c dag.Config[ComplexState]) (dag.NodeResponse[ComplexState], error) {
		return dag.NodeResponse[ComplexState]{State: s, Status: dag.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("producer1", func(ctx context.Context, s ComplexState, c dag.Config[ComplexState]) (dag.NodeResponse[ComplexState], error) {
		s.Numbers = append(s.Numbers, 1, 2, 3)
		err := numbersChannel.Write(ctx, s, c)
		return dag.NodeResponse[ComplexState]{State: s, Status: dag.StatusCompleted}, err
	}, nil))

	require.NoError(t, g.AddNode("producer2", func(ctx context.Context, s ComplexState, c dag.Config[ComplexState]) (dag.NodeResponse[ComplexState], error) {
		// Read previous state from channel
		prevState, err := numbersChannel.Read(ctx, c)
		if err != nil {
			return dag.NodeResponse[ComplexState]{State: s, Status: dag.StatusCompleted}, err
		}

		// Append new numbers to existing ones
		s.Numbers = append(prevState.Numbers, 4, 5, 6)
		err = numbersChannel.Write(ctx, s, c)
		return dag.NodeResponse[ComplexState]{State: s, Status: dag.StatusCompleted}, err
	}, nil))

	require.NoError(t, g.AddNode("consumer", func(ctx context.Context, s ComplexState, c dag.Config[ComplexState]) (dag.NodeResponse[ComplexState], error) {
		state, err := numbersChannel.Read(ctx, c)
		if err != nil {
			return dag.NodeResponse[ComplexState]{State: s, Status: dag.StatusCompleted}, err
		}
		s.Numbers = state.Numbers
		s.Done = true
		return dag.NodeResponse[ComplexState]{State: s, Status: dag.StatusCompleted}, nil
	}, nil))

	// Add sequential path
	require.NoError(t, g.AddEdge("start", "producer1", nil))
	require.NoError(t, g.AddEdge("producer1", "producer2", nil))
	require.NoError(t, g.AddEdge("producer2", "consumer", nil))
	require.NoError(t, g.AddEdge("consumer", dag.END, nil))

	require.NoError(t, g.SetEntryPoint("start"))

	compiled, err := g.Compile(dag.WithDebug[ComplexState]())
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)}, dag.WithThreadID[ComplexState]("test-channels"))
	require.NoError(t, err)
	assert.True(t, result.Done)
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6}, result.Numbers)
}

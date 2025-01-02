package channels

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/avi3tal/orchestrai/internal/graph"
	"github.com/avi3tal/orchestrai/internal/state"
	"github.com/avi3tal/orchestrai/internal/types"
	"github.com/stretchr/testify/require"
)

type TestState struct {
	Value string
}

func (s TestState) Validate() error {
	return nil
}

func (s TestState) Merge(other state.GraphState) state.GraphState {
	o, ok := other.(TestState)
	if !ok {
		return s
	}
	if o.Value != "" {
		s.Value = o.Value
	}
	return s
}

func (s TestState) Clone() state.GraphState {
	return TestState{Value: s.Value}
}

func (s TestState) Dump() ([]byte, error) {
	return []byte(s.Value), nil
}

func (s TestState) Load(data []byte) (state.GraphState, error) {
	return TestState{Value: string(data)}, nil
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

func TestLastValueChannel(t *testing.T) {
	t.Parallel()

	t.Run("Write and Read", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		ch := NewLastValue()
		cfg := types.Config{ThreadID: "test"}
		st := TestState{Value: "test"}
		err := ch.Write(ctx, st, cfg)
		require.NoError(t, err)

		got, err := ch.Read(ctx, cfg)
		require.NoError(t, err)
		require.Equal(t, st, got)
	})

	t.Run("Multiple Writes", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		ch := NewLastValue()
		cfg := types.Config{ThreadID: "test"}
		states := []TestState{
			{Value: "first"},
			{Value: "second"},
			{Value: "third"},
		}

		for _, s := range states {
			err := ch.Write(ctx, s, cfg)
			require.NoError(t, err)
		}

		got, err := ch.Read(ctx, cfg)
		require.NoError(t, err)
		require.Equal(t, states[len(states)-1], got)
	})
}

func TestBarrierChannel(t *testing.T) {
	t.Parallel()

	t.Run("Write Before All Required", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		required := []string{"node1", "node2"}
		ch := NewBarrierChannel(required)
		st := TestState{Value: "test"}
		err := ch.Write(ctx, st, types.Config{ThreadID: "node1"})
		require.NoError(t, err)

		_, err = ch.Read(ctx, types.Config{})
		require.Error(t, err)
	})

	t.Run("Write All Required", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		required := []string{"node1", "node2"}
		ch := NewBarrierChannel(required)
		state1 := TestState{Value: "node1"}
		state2 := TestState{Value: "node2"}

		err := ch.Write(ctx, state1, types.Config{ThreadID: "node1"})
		require.NoError(t, err)

		err = ch.Write(ctx, state2, types.Config{ThreadID: "node2"})
		require.NoError(t, err)

		got, err := ch.Read(ctx, types.Config{})
		require.NoError(t, err)
		require.Equal(t, state2, got)
	})

	t.Run("Invalid Node Write", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		required := []string{"node1", "node2"}
		ch := NewBarrierChannel(required)
		err := ch.Write(ctx, TestState{}, types.Config{ThreadID: "invalid"})
		require.Error(t, err)
	})
}

func TestDynamicBarrierChannel(t *testing.T) {
	t.Parallel()

	t.Run("Dynamic Registration", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		ch := NewDynamicBarrierChannel()
		ch.AddRequired("node1")
		ch.AddRequired("node2")

		st := TestState{Value: "test"}
		err := ch.Write(ctx, st, types.Config{ThreadID: "node1"})
		require.NoError(t, err)

		_, err = ch.Read(ctx, types.Config{})
		require.Error(t, err)

		err = ch.Write(ctx, st, types.Config{ThreadID: "node2"})
		require.NoError(t, err)

		got, err := ch.Read(ctx, types.Config{})
		require.NoError(t, err)
		require.Equal(t, st, got)
	})

	t.Run("Concurrent Access", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		ch := NewDynamicBarrierChannel()
		nodes := []string{"node1", "node2", "node3"}

		for _, node := range nodes {
			ch.AddRequired(node)
		}

		errCh := make(chan error, len(nodes))

		for _, node := range nodes {
			nodeID := node
			go func() {
				err := ch.Write(ctx, TestState{Value: nodeID}, types.Config{ThreadID: nodeID})
				errCh <- err
			}()
		}

		for range nodes {
			err := <-errCh
			require.NoError(t, err)
		}

		got, err := ch.Read(ctx, types.Config{})
		gt, ok := got.(TestState)
		require.True(t, ok)
		require.NoError(t, err)
		require.NotEmpty(t, gt.Value)
	})
}

// Test graph with channels
func TestChannelGraphExecution(t *testing.T) {
	t.Parallel()
	g := graph.NewGraph("channel-graph")

	numbersChannel := NewLastValue()
	// Add channels
	require.NoError(t, g.AddChannel("numbers", numbersChannel))

	// Add nodes
	require.NoError(t, g.AddNode("start", func(_ context.Context, other state.GraphState, _ types.Config) (graph.NodeResponse, error) {
		return graph.NodeResponse{State: other, Status: types.StatusCompleted}, nil
	}, nil))

	require.NoError(t, g.AddNode("producer1", func(ctx context.Context, other state.GraphState, c types.Config) (graph.NodeResponse, error) {
		o, ok := other.(ComplexState)
		if !ok {
			return graph.NodeResponse{State: other, Status: types.StatusCompleted}, errors.New("invalid state type")
		}

		o.Numbers = append(o.Numbers, 1, 2, 3)
		err := numbersChannel.Write(ctx, o, c)
		return graph.NodeResponse{State: o, Status: types.StatusCompleted}, err
	}, nil))

	require.NoError(t, g.AddNode("producer2", func(ctx context.Context, other state.GraphState, c types.Config) (graph.NodeResponse, error) {
		// Read previous state from channel
		prevState, err := numbersChannel.Read(ctx, c)
		ps, ok := prevState.(ComplexState)
		if !ok {
			return graph.NodeResponse{State: other, Status: types.StatusCompleted}, errors.New("invalid state type")
		}
		if err != nil {
			return graph.NodeResponse{State: other, Status: types.StatusCompleted}, err
		}

		o, ok := other.(ComplexState)
		if !ok {
			return graph.NodeResponse{State: other, Status: types.StatusCompleted}, errors.New("invalid state type")
		}

		// Append new numbers to existing ones
		copy(o.Numbers, ps.Numbers)
		o.Numbers = append(o.Numbers, 4, 5, 6)
		err = numbersChannel.Write(ctx, o, c)
		return graph.NodeResponse{State: o, Status: types.StatusCompleted}, err
	}, nil))

	require.NoError(t, g.AddNode("consumer", func(ctx context.Context, other state.GraphState, c types.Config) (graph.NodeResponse, error) {
		prevState, err := numbersChannel.Read(ctx, c)
		ps, ok := prevState.(ComplexState)
		if !ok {
			return graph.NodeResponse{State: other, Status: types.StatusCompleted}, errors.New("invalid state type")
		}
		if err != nil {
			return graph.NodeResponse{State: other, Status: types.StatusCompleted}, err
		}

		o, ok := other.(ComplexState)
		if !ok {
			return graph.NodeResponse{State: other, Status: types.StatusCompleted}, errors.New("invalid state type")
		}

		o.Numbers = ps.Numbers
		o.Done = true
		return graph.NodeResponse{State: o, Status: types.StatusCompleted}, nil
	}, nil))

	// Add sequential path
	require.NoError(t, g.AddEdge("start", "producer1", nil))
	require.NoError(t, g.AddEdge("producer1", "producer2", nil))
	require.NoError(t, g.AddEdge("producer2", "consumer", nil))
	require.NoError(t, g.AddEdge("consumer", graph.END, nil))

	require.NoError(t, g.SetEntryPoint("start"))

	compiled, err := g.Compile(graph.WithDebug())
	require.NoError(t, err)

	result, err := compiled.Run(context.Background(), ComplexState{Numbers: make([]int, 0)}, graph.WithThreadID("test-channels"))
	require.NoError(t, err)
	r, ok := result.(ComplexState)
	require.True(t, ok)
	require.True(t, r.Done)
	require.Equal(t, []int{1, 2, 3, 4, 5, 6}, r.Numbers)
}

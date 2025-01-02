package checkpoints

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/avi3tal/orchestrai/internal/state"
	"github.com/avi3tal/orchestrai/internal/types"
	"github.com/stretchr/testify/require"
)

type TestState struct {
	Value int
}

func (s TestState) Validate() error {
	if s.Value < 0 {
		return errors.New("value cannot be negative")
	}
	return nil
}

func (s TestState) Merge(other state.GraphState) state.GraphState {
	o, _ := other.(TestState)
	return TestState{Value: s.Value + o.Value}
}

func (s TestState) Clone() state.GraphState {
	return TestState{Value: s.Value}
}

func (s TestState) Dump() ([]byte, error) {
	return json.Marshal(s)
}

func (s TestState) Load(data []byte) (state.GraphState, error) {
	var st TestState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return st, nil
}

// Test state validation
func TestStateValidation(t *testing.T) {
	t.Parallel()
	st := TestState{Value: 5}
	require.NoError(t, st.Validate())

	invalidState := TestState{Value: -1}
	require.Error(t, invalidState.Validate())
}

// Test state merging
func TestStateMerge(t *testing.T) {
	t.Parallel()
	state1 := TestState{Value: 5}
	state2 := TestState{Value: 3}
	mergedState := state1.Merge(state2)
	s, _ := mergedState.(TestState)
	require.Equal(t, 8, s.Value)
}

func TestMemoryCheckpointer(t *testing.T) {
	t.Parallel()
	checkpointer := NewStateCheckpointer(NewMemoryStore())

	ctx := context.Background()
	config := types.Config{ThreadID: "thread-1"}
	data := &types.DataPoint{
		State:       TestState{Value: 10},
		Status:      types.StatusCompleted,
		CurrentNode: "node1",
	}

	// Save checkpoint
	err := checkpointer.Save(ctx, config, data)
	require.NoError(t, err)

	// Load checkpoint
	loadedData, err := checkpointer.Load(ctx, config)
	require.NoError(t, err)
	require.Equal(t, data, loadedData)
}

func TestMemoryCheckpointerEdgeCases(t *testing.T) {
	t.Parallel()
	checkpointer := NewStateCheckpointer(NewMemoryStore())
	ctx := context.Background()

	// Test loading nonexistent checkpoint
	config1 := types.Config{ThreadID: "thread-1"}
	_, err := checkpointer.Load(ctx, config1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "thread-1")
	require.Contains(t, err.Error(), "checkpoint not found")

	// Test overwriting checkpoints
	data1 := &types.DataPoint{
		State:       TestState{Value: 10},
		Status:      types.StatusCompleted,
		CurrentNode: "node1",
	}
	err = checkpointer.Save(ctx, config1, data1)
	require.NoError(t, err)

	data2 := &types.DataPoint{
		State:       TestState{Value: 20},
		Status:      types.StatusPending,
		CurrentNode: "node2",
	}
	err = checkpointer.Save(ctx, config1, data2)
	require.NoError(t, err)

	loadedData, err := checkpointer.Load(ctx, config1)
	require.NoError(t, err)
	require.Equal(t, data2, loadedData)

	// Test thread safety with concurrent access
	config2 := types.Config{ThreadID: "thread-2"}
	data3 := &types.DataPoint{
		State:       TestState{Value: 30},
		Status:      types.StatusCompleted,
		CurrentNode: "node3",
	}
	err = checkpointer.Save(ctx, config2, data3)
	require.NoError(t, err)
	res, err := checkpointer.Load(ctx, config2)
	require.NoError(t, err)
	require.Equal(t, data3, res)
}

func TestStateCheckpointer(t *testing.T) {
	t.Parallel()
	checkpointer := NewStateCheckpointer(NewMemoryStore())
	ctx := context.Background()

	// Test saving and loading checkpoints
	config := types.Config{ThreadID: "thread-1"}
	data := &types.DataPoint{
		State:       TestState{Value: 100},
		Status:      types.StatusPending,
		CurrentNode: "node1",
	}

	// Save checkpoint
	err := checkpointer.Save(ctx, config, data)
	require.NoError(t, err)

	// Load checkpoint
	loadedData, err := checkpointer.Load(ctx, config)
	require.NoError(t, err)
	require.Equal(t, data, loadedData)

	// Test loading nonexistent checkpoint
	nonexistentConfig := types.Config{ThreadID: "thread-2"}
	_, err = checkpointer.Load(ctx, nonexistentConfig)
	require.Error(t, err)
	require.Contains(t, err.Error(), "thread-2")
	require.Contains(t, err.Error(), "checkpoint not found")
}
